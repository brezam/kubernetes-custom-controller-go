package cloudamqp

import (
	"bruno-zamariola/kubernetes-custom-controller/resourcesrestapi/rabbit"
	"fmt"
	"regexp"

	"bruno-zamariola/kubernetes-custom-controller/util"

	"github.com/go-resty/resty/v2"
)

type CloudAMQPAPI struct {
	apikey string
	client *resty.Client
}

// This is the response value of each instance in a GET at /api/instances
type SimpleInstance struct {
	Id          int      `json:"id"`
	Name        string   `json:"name"`
	Plan        string   `json:"plan"`
	Region      string   `json:"region"`
	Tags        []string `json:"tags"`
	ProviderIid string   `json:"providerid"`
	VpcId       string   `json:"vpc_id"`
}

// This is the response value of a single instance when GET at /api/instances/<id>
type DetailedInstance struct {
	Id               int                   `json:"id"`
	Name             string                `json:"name"`
	Plan             string                `json:"plan"`
	Region           string                `json:"region"`
	Tags             []string              `json:"tags"`
	ProviderIid      string                `json:"providerid"`
	Url              string                `json:"url"`
	Ready            bool                  `json:"ready"`
	Apikey           string                `json:"apikey"`
	Backend          string                `json:"backend"`
	Nodes            int                   `json:"nodes"`
	RmqVersion       string                `json:"rmq_version"`
	Urls             *DetailedInstanceUrls `json:"urls"`
	HostnameExternal string                `json:"hostname_external"`
	HostnameInternal string                `json:"hostname_internal"`
}

type DetailedInstanceUrls struct {
	External string `json:"external"`
	Internal string `json:"internal"`
}

func New(apikey string) *CloudAMQPAPI {
	client := resty.New()
	client.SetBaseURL("https://customer.cloudamqp.com/api")
	client.SetBasicAuth("", apikey)
	return &CloudAMQPAPI{apikey: apikey, client: client}
}

func (c *CloudAMQPAPI) GetAllInstances() (*[]SimpleInstance, error) {
	resp, err := c.client.R().
		SetResult([]SimpleInstance{}).
		Get("instances")
	if err != nil {
		return nil, err
	}
	return resp.Result().(*[]SimpleInstance), nil
}

func (c *CloudAMQPAPI) GetInstanceFromTags(tags []string) (*rabbit.RabbitAPI, error) {
	allInstances, err := c.GetAllInstances()
	if err != nil {
		return nil, err
	}
	var selectedInstance *rabbit.RabbitAPI
	for _, instance := range *allInstances {
		if !util.ContainsAll(instance.Tags, tags) {
			continue
		}
		selectedInstance, err = c.GetDetailedInstanceInfo(instance.Id)
		if err != nil {
			return nil, err
		}
	}
	if selectedInstance == nil {
		return nil, fmt.Errorf("could not find instance with tags %v", tags)
	}
	return selectedInstance, nil
}

func (c *CloudAMQPAPI) GetDetailedInstanceInfo(id int) (*rabbit.RabbitAPI, error) {
	resp, err := c.client.R().
		SetResult(&DetailedInstance{}).
		Get(fmt.Sprintf("instances/%d", id))
	if err != nil {
		return nil, err
	}
	body := resp.Result().(*DetailedInstance)
	amqpUrlPattern := regexp.MustCompile("amqps://(.*):(.*)@(.*)/")
	matches := amqpUrlPattern.FindStringSubmatch(body.Urls.External)
	return rabbit.NewRabbitAPI(body.HostnameExternal, matches[1], matches[2]), nil
}
