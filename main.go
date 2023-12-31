/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"flag"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"

	// Uncomment the following line to load the gcp plugin (only required to authenticate against GKE clusters).
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"

	clientset "bruno-zamariola/kubernetes-custom-controller/pkg/generated/clientset/versioned"
	informers "bruno-zamariola/kubernetes-custom-controller/pkg/generated/informers/externalversions"
	"bruno-zamariola/kubernetes-custom-controller/pkg/signals"
	"bruno-zamariola/kubernetes-custom-controller/resourcesrestapi/cloudamqpbar"
)

var (
	// CloudAMQP credential flags
	bar string
	// Custom controller resync flags
	resyncDuration time.Duration
	// Kubernetes cluster connection flags
	masterURL  string
	kubeconfig string
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	// Set up signals so we handle the shutdown signal gracefully
	ctx := signals.SetupSignalHandler()
	logger := klog.FromContext(ctx)

	// Setting up CloudAMQP api
	cloudamqpBarApi, err := cloudamqpbar.New(bar)
	if err != nil {
		logger.Error(err, "Error building cloudamqp api")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	cfg, err := clientcmd.BuildConfigFromFlags(masterURL, kubeconfig)
	if err != nil {
		logger.Error(err, "Error building kubeconfig")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Error(err, "Error building kubernetes clientset")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	controllerClient, err := clientset.NewForConfig(cfg)
	if err != nil {
		logger.Error(err, "Error building custom controller clientset")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}

	kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, resyncDuration)
	customControllerInformerFactory := informers.NewSharedInformerFactory(controllerClient, resyncDuration)

	controller := NewController(ctx, cloudamqpBarApi, kubeClient, controllerClient,
		kubeInformerFactory.Core().V1().Secrets(),
		customControllerInformerFactory.Brunoz().V1alpha1().Rabbits())

	// Notice that there is no need to run Start methods in a separate goroutine. (i.e. go kubeInformerFactory.Start(ctx.done())
	// Start method is non-blocking and runs all registered informers in a dedicated goroutine.
	kubeInformerFactory.Start(ctx.Done())
	customControllerInformerFactory.Start(ctx.Done())

	if err = controller.Run(ctx, 2); err != nil {
		logger.Error(err, "Error running controller")
		klog.FlushAndExit(klog.ExitFlushTimeout, 1)
	}
}

func init() {
	flag.StringVar(&bar, "bar", "", "Bar.")
	flag.DurationVar(&resyncDuration, "resyncPeriod", 1*time.Minute, "Resync period in for custom controller informer.")
	flag.StringVar(&kubeconfig, "kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
	flag.StringVar(&masterURL, "master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
}
