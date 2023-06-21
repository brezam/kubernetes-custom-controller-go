package main

import (
	samplev1alpha1 "bruno-zamariola/kubernetes-custom-controller/pkg/apis/rabbit/v1alpha1"
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

// handleSecretResrouce will take any resource implementing metav1.Object and attempt
// to find the Rabbit resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Rabbit resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleSecretResource(obj interface{}) {
	var object metav1.Object
	var ok bool
	logger := klog.FromContext(context.Background())
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		logger.V(4).Info("Recovered deleted object", "resourceName", object.GetName())
	}
	logger.V(4).Info("Processing object", "object", klog.KObj(object))
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Rabbit, we should not do anything more
		// with it.
		if ownerRef.Kind != "Rabbit" {
			return
		}

		rabbit, err := c.rabbitLister.Rabbits(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			logger.V(4).Info("Ignore orphaned object", "object", klog.KObj(object), "rabbit", ownerRef.Name)
			return
		}

		c.enqueueRabbit(rabbit)
		return
	}
}

// newSecret creates a new Secret for a rabbit resource. It also sets
// the appropriate OwnerReferences on the resource so handleObject can discover
// the Rabbit resource that 'owns' it.
func newSecret(secretName, host, password string, rabbit *samplev1alpha1.Rabbit) *corev1.Secret {
	labels := map[string]string{
		"controller": rabbit.Name,
	}
	prefix := "RABBITMQ_" + strings.ToUpper(strings.Replace(rabbit.Spec.InstanceType, "-", "_", -1))
	data := map[string][]byte{
		prefix + "_HOST":     []byte(host),
		prefix + "_USERNAME": []byte(rabbit.Spec.Username),
		prefix + "_PASSWORD": []byte(password),
	}
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: rabbit.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(rabbit, samplev1alpha1.SchemeGroupVersion.WithKind("Rabbit")),
			},
			Labels: labels,
		},
		Data: data,
	}
}
