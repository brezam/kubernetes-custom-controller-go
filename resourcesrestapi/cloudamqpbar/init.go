package cloudamqpbar

import (
	_ "embed"
	"encoding/json"
)

//go:embed cloudamqp-bar-credentials.json
var credentialsBytes []byte
var credentials map[string]string

func init() {
	json.Unmarshal(credentialsBytes, &credentials)
}
