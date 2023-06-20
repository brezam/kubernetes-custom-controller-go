package main

import (
	samplev1alpha1 "bruno-zamariola/kubernetes-custom-controller/pkg/apis/rabbit/v1alpha1"
	"bruno-zamariola/kubernetes-custom-controller/resourcesrestapi/rabbit"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

type RabbitUserCreation struct {
	rabbit    *samplev1alpha1.Rabbit
	username  string
	password  string
	vhost     string
	tags      string
	read      string
	write     string
	configure string
}

func (c *Controller) createRabbitUserAndSetPermissions(ctx context.Context, rabbitapi *rabbit.RabbitAPI, data *RabbitUserCreation) error {
	logger := klog.FromContext(ctx)
	// Creating RabbitMQ user
	success, err := rabbitapi.CreateUser(data.username, data.password, data.tags)
	if err != nil {
		msg := fmt.Sprintf(FailureToCreateRabbitUser, data.username)
		c.recorder.Event(data.rabbit, corev1.EventTypeWarning, ErrRabbitAPIFailure, msg)
		return err
	}
	if !success {
		msg := fmt.Sprintf(FailureToCreateRabbitUser, data.username)
		c.recorder.Event(data.rabbit, corev1.EventTypeWarning, ErrRabbitAPIFailure, msg)
		return fmt.Errorf(msg)
	}
	// Creating RabbitMQ user permissions
	logger.Info("Creating permissions", "read", data.read, "write", data.write, "configure", data.configure)
	success, err = rabbitapi.CreatePermissions(data.username, data.vhost, data.read, data.write, data.configure)
	if err != nil {
		msg := fmt.Sprintf(FailureToCreateRabbitPermissions, data.username)
		c.recorder.Event(data.rabbit, corev1.EventTypeWarning, ErrRabbitAPIFailure, msg)
		return err
	}
	if !success {
		msg := fmt.Sprintf(FailureToCreateRabbitPermissions, data.username)
		c.recorder.Event(data.rabbit, corev1.EventTypeWarning, ErrRabbitAPIFailure, msg)
		return fmt.Errorf(msg)
	}
	return nil
}
