package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Rabbit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RabbitSpec   `json:"spec"`
	Status RabbitStatus `json:"status"`
}

type RabbitSpec struct {
	InstanceType string             `json:"instanceType"`
	Vhost        string             `json:"vhost"`
	Username     string             `json:"username"`
	Password     *RabbitPassword    `json:"password"`
	Permissions  *RabbitPermissions `json:"permissions,omitempty"`
}

type RabbitPassword struct {
	Type  string `json:"type"`
	Value string `json:"value,omitempty"`
}

type RabbitPermissions struct {
	Read      string `json:"read,omitempty"`
	Write     string `json:"write,omitempty"`
	Configure string `json:"configure,omitempty"`
}

type RabbitStatus struct {
	Error        bool   `json:"error"`
	ErrorMessage string `json:"errorMessage"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RabbitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Rabbit `json:"items"`
}
