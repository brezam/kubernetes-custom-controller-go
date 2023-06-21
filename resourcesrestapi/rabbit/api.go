package rabbit

import (
	"fmt"

	"github.com/go-resty/resty/v2"
)

type RabbitAPI struct {
	Host   string
	client *resty.Client
}

type UserCreationRequest struct {
	Password string `json:"password"`
	Tags     string `json:"tags"`
}

type PermissionRequest struct {
	Read      string `json:"read"`
	Write     string `json:"write"`
	Configure string `json:"configure"`
}

func NewRabbitAPI(host, authUser, authPassword string) *RabbitAPI {
	client := resty.New()
	client.SetBasicAuth(authUser, authPassword)
	client.SetBaseURL(fmt.Sprintf("https://%s/api", host))
	return &RabbitAPI{Host: host, client: client}
}

func (r *RabbitAPI) CreateUser(username, password, tags string) (bool, error) {
	resp, err := r.client.R().
		SetBody(&UserCreationRequest{
			Password: password,
			Tags:     tags,
		}).
		SetHeader("Content-Type", "application/json").
		Put(fmt.Sprintf("users/%s", username))
	if err != nil {
		return false, err
	}
	return resp.IsSuccess(), nil
}

func (r *RabbitAPI) CreatePermissions(user, vhost, read, write, configure string) (bool, error) {
	resp, err := r.client.R().
		SetBody(&PermissionRequest{
			Read:      read,
			Write:     write,
			Configure: configure,
		}).
		Put(fmt.Sprintf("permissions/%s/%s", vhost, user))
	if err != nil {
		return false, err
	}
	return resp.IsSuccess(), nil
}
