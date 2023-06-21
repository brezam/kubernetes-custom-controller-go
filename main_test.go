package main

import (
	"bruno-zamariola/kubernetes-custom-controller/resourcesrestapi/cloudamqpbar"
	"testing"
)

func TestApi(t *testing.T) {
	c, err := cloudamqpbar.New("shared-sit")
	if err != nil {
		t.Error(err)
	}
	instance, err := c.GetInstanceInBarFromTags([]string{"shared"})
	if err != nil {
		t.Error(err)
	}
	expectedHost := "serious-lobster.rmq.cloudamqp.com"
	if !(instance.Host == expectedHost) {
		t.Errorf("host did not match\nExpected:%s\nActual:%s", expectedHost, instance.Host)
	}
}
