package cloudamqpbar

import (
	"bruno-zamariola/kubernetes-custom-controller/resourcesrestapi/cloudamqp"
	"bruno-zamariola/kubernetes-custom-controller/resourcesrestapi/rabbit"
	"fmt"
)

type CloudAMQPBarAPI struct {
	bar string
	api *cloudamqp.CloudAMQPAPI
}

func New(bar string) (*CloudAMQPBarAPI, error) {
	apikey, ok := credentials[bar]
	if !ok {
		return nil, fmt.Errorf("could not find bar in credentials.json")
	}
	return &CloudAMQPBarAPI{bar: bar, api: cloudamqp.New(apikey)}, nil
}

func (c *CloudAMQPBarAPI) GetInstanceInBarFromTags(tags []string) (*rabbit.RabbitAPI, error) {
	return c.api.GetInstanceFromTags(append(tags, c.bar))
}
