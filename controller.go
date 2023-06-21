package main

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"

	samplev1alpha1 "bruno-zamariola/kubernetes-custom-controller/pkg/apis/rabbit/v1alpha1"
	clientset "bruno-zamariola/kubernetes-custom-controller/pkg/generated/clientset/versioned"
	samplescheme "bruno-zamariola/kubernetes-custom-controller/pkg/generated/clientset/versioned/scheme"
	informers "bruno-zamariola/kubernetes-custom-controller/pkg/generated/informers/externalversions/rabbit/v1alpha1"
	listers "bruno-zamariola/kubernetes-custom-controller/pkg/generated/listers/rabbit/v1alpha1"
	"bruno-zamariola/kubernetes-custom-controller/util"

	"bruno-zamariola/kubernetes-custom-controller/resourcesrestapi/cloudamqpbar"
)

const controllerAgentName = "rabbit"

const (
	ErrSpecMisconfigured     = "ErrSpecDefinition"
	SpecIsMisconfigured      = "Spec is misconfigured"
	PermissionFieldsAreBlank = "Permission fields are blank"

	ErrRabbitAPIFailure              = "ErrRabbitAPIFailure"
	FailureToCreateRabbitUser        = "Failure to create rabbit user %s"
	FailureToCreateRabbitPermissions = "Failure to create rabbit permissions for user %s"
	CouldNotFindRabbit               = "Could not find rabbit with instance type %s"

	// SuccessSynced is used as part of the Event 'reason' when a Rabbit is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Rabbit fails
	// to sync due to a Deployment of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Deployment already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Rabbit"
	// MessageResourceSynced is the message used for an Event fired when a Rabbit
	// is synced successfully
	MessageResourceSynced = "Rabbit synced successfully"
)

// Controller is the controller implementation for Rabbit resources
type Controller struct {
	cloudamqpBarApi *cloudamqpbar.CloudAMQPBarAPI

	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// sampleclientset is a clientset for our own API group
	sampleclientset clientset.Interface

	secretsLister corelisters.SecretLister
	secretsSynced cache.InformerSynced
	rabbitLister  listers.RabbitLister
	rabbitSynced  cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new sample controller
func NewController(
	ctx context.Context,
	cloudamqpBarApi *cloudamqpbar.CloudAMQPBarAPI,
	kubeclientset kubernetes.Interface,
	sampleclientset clientset.Interface,
	secretInformer coreinformers.SecretInformer,
	rabbitInformer informers.RabbitInformer) *Controller {
	logger := klog.FromContext(ctx)

	// Create event broadcaster
	// Add sample-controller types to the default Kubernetes Scheme so Events can be
	// logged for sample-controller types.
	utilruntime.Must(samplescheme.AddToScheme(scheme.Scheme))
	logger.V(4).Info("Creating event broadcaster")

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeclientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		cloudamqpBarApi: cloudamqpBarApi,
		kubeclientset:   kubeclientset,
		sampleclientset: sampleclientset,
		secretsLister:   secretInformer.Lister(),
		secretsSynced:   secretInformer.Informer().HasSynced,
		rabbitLister:    rabbitInformer.Lister(),
		rabbitSynced:    rabbitInformer.Informer().HasSynced,
		workqueue:       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Rabbits"),
		recorder:        recorder,
	}

	logger.Info("Setting up event handlers")
	// Set up an event handler for when Rabbit resources change
	rabbitInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueRabbit,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueRabbit(new)
		},
	})
	// Set up an event handler for when Secrets resources change. This
	// handler will lookup the owner of the given Secret, and if it is
	// owned by a Rabbit resource then the handler will enqueue that Rabbit resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Secret resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleSecretResource,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*corev1.Secret)
			oldDepl := old.(*corev1.Secret)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Secrets.
				// Two different versions of the same Secret will always have different RVs.
				return
			}
			controller.handleSecretResource(new)
		},
		DeleteFunc: controller.handleSecretResource,
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(ctx context.Context, workers int) error {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()
	logger := klog.FromContext(ctx)

	// Start the informer factories to begin populating the informer caches
	logger.Info("Starting Rabbit controller")

	// Wait for the caches to be synced before starting workers
	logger.Info("Waiting for informer caches to sync")

	if ok := cache.WaitForCacheSync(ctx.Done(), c.secretsSynced, c.rabbitSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logger.Info("Starting workers", "count", workers)
	// Launch two workers to process Rabbit resources
	for i := 0; i < workers; i++ {
		go wait.UntilWithContext(ctx, c.runWorker, time.Second)
	}

	logger.Info("Started workers")
	<-ctx.Done()
	logger.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem(ctx context.Context) bool {
	obj, shutdown := c.workqueue.Get()
	logger := klog.FromContext(ctx)

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Rabbit resource to be synced.
		if err := c.syncHandler(ctx, key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		logger.Info("Successfully synced", "resourceName", key)
		return nil
	}(obj)

	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Rabbit resource
// with the current status of the resource.
func (c *Controller) syncHandler(ctx context.Context, key string) error {
	logger := klog.LoggerWithValues(klog.FromContext(ctx), "resourceName", key)

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Rabbit resource with this namespace/name
	rabbit, err := c.rabbitLister.Rabbits(namespace).Get(name)
	if err != nil {
		// The Rabbit resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			utilruntime.HandleError(fmt.Errorf("rabbit '%s' in work queue no longer exists", key))
			return nil
		}
		return err
	}

	// Validating object
	instanceType := rabbit.Spec.InstanceType
	vhost := rabbit.Spec.Vhost
	username := rabbit.Spec.Username
	permissions := rabbit.Spec.Permissions
	if permissions == nil {
		permissions = &samplev1alpha1.RabbitPermissions{Read: ".*", Write: ".*", Configure: ".*"}
	}
	if permissions.Read == "" || permissions.Write == "" || permissions.Configure == "" {
		c.updateRabbitStatus(rabbit, true, ErrSpecMisconfigured)
		c.recorder.Event(rabbit, corev1.EventTypeWarning, ErrSpecMisconfigured, PermissionFieldsAreBlank)
		utilruntime.HandleError(fmt.Errorf("%s: permission fields cannot be blank", key))
		return nil
	}

	rabbitapi, err := c.cloudamqpBarApi.GetInstanceInBarFromTags([]string{rabbit.Spec.InstanceType})
	if err != nil {
		c.updateRabbitStatus(rabbit, true, ErrRabbitAPIFailure)
		c.recorder.Event(rabbit, corev1.EventTypeWarning, ErrRabbitAPIFailure, err.Error())
		return err
	}
	// Try to get the secret if it exists
	secretName := fmt.Sprintf("rabbitmq-%s-user-%s", instanceType, username)
	secret, err := c.secretsLister.Secrets(namespace).Get(secretName)

	// If the secret resource doesn't exist, we'll create the RabbitMQ user and create the secret
	var password string
	if rabbit.Spec.Password.Type == "random" {
		password = util.GenerateRandomString(24)
	} else {
		password = rabbit.Spec.Password.Value
	}
	if errors.IsNotFound(err) {
		tags := "management"
		logger.Info("Creating user", "InstanceType", instanceType, "Username", username, "Tags", tags, "Password", password[0:3]+"...")
		c.createRabbitUserAndSetPermissions(ctx, rabbitapi, &RabbitUserCreation{
			rabbit:    rabbit,
			username:  username,
			password:  password,
			vhost:     vhost,
			tags:      tags,
			read:      permissions.Read,
			write:     permissions.Write,
			configure: permissions.Configure,
		})
		secret, err = c.kubeclientset.CoreV1().Secrets(rabbit.Namespace).
			Create(context.TODO(), newSecret(secretName, rabbitapi.Host, password, rabbit), metav1.CreateOptions{})
		// If an error occurs during Get/Create, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if err != nil {
			return err
		}
	}

	// If the Secret is not controlled by this Rabbit resource, we should log
	// a warning to the event recorder and return error msg.
	if !metav1.IsControlledBy(secret, rabbit) {
		msg := fmt.Sprintf(MessageResourceExists, secret.Name)
		c.recorder.Event(rabbit, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf("%s", msg)
	}

	// Finally, we update the status block of the Rabbit resource to reflect the
	// current state of the world
	err = c.updateRabbitStatus(rabbit, false, "")
	if err != nil {
		return err
	}

	c.recorder.Event(rabbit, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateRabbitStatus(rabbit *samplev1alpha1.Rabbit, isErr bool, errMsg string) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	rabbitCopy := rabbit.DeepCopy()
	rabbitCopy.Status.Error = isErr
	rabbitCopy.Status.ErrorMessage = errMsg
	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Rabbit resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.sampleclientset.BrunozV1alpha1().Rabbits(rabbit.Namespace).UpdateStatus(context.TODO(), rabbitCopy, metav1.UpdateOptions{})
	return err
}

// enqueueRabbit takes a Rabbit resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Rabbit.
func (c *Controller) enqueueRabbit(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}
