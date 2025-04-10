package main

import (
	// "encoding/base64"
	"fmt"
	"strings"
)

// ECSTaskConfig contains configuration values to trigger the AWS ECS RunTask API
type ECSTaskConfig struct {
	Cluster        string   // The cluster name
	TaskDefinition string   // The family and revision ( family:revision ) or full ARN of the task definition to run. If a revision isn't specified, the latest ACTIVE revision is used
	ClientToken    string   // A client token for idempotent requests to the AWS ECS RunTask API
	Subnets        []string // List of subnet IDs
	SecurityGroups []string // List of security group IDs
}

// ECSTaskReadConfig contains configuration values to read information about a single task from AWS ECS
type ECSTaskReadConfig struct {
	Cluster string // The cluster name
	TaskARN string // The task ARN
}

/*
ReadFromEnv reads the following required environment variables
and populates the struct with the values:
  - ECS_CLUSTER: The ECS cluster name
  - ECS_TASK_DEFINITION: The family and revision ( family:revision ) or full ARN of the task definition to run. If a revision isn't specified, the latest ACTIVE revision is used
  - SUBNET_IDS: A comma-separated list of subnet IDs
  - SECURITY_GROUP_IDS: A comma-separated list of security group IDs
*/
func (config *ECSTaskConfig) ReadFromEnv() {
	config.Cluster = ReadRequiredEnvVar("ECS_CLUSTER")
	config.TaskDefinition = ReadRequiredEnvVar("ECS_TASK_DEFINITION")

	subnetIDsStr := ReadRequiredEnvVar("SUBNET_IDS")
	config.Subnets = strings.Split(subnetIDsStr, ",")

	securityGroupIDsStr := ReadRequiredEnvVar("SECURITY_GROUP_IDS")
	config.SecurityGroups = strings.Split(securityGroupIDsStr, ",")
}

/*
SetClientToken populates the ClientToken field with a
well-formatted value generated from an input string.
The client token is used for idempotent requests to the AWS ECS RunTask API.

See:

https://docs.aws.amazon.com/AmazonECS/latest/APIReference/ECS_Idempotency.html#RunTaskIdempotency
*/
func (config *ECSTaskConfig) SetClientToken(input string) {
	config.ClientToken = GenerateClientToken(input)
}

/*
ADOPayload contains a parsed JSON payload sent
from an Azure DevOps 'Generic' service connection check of type 'Invoke REST API'.
*/
type ADOPayload struct {
	PlanURL        string `json:"PlanUrl"`        // The plan URL (system.CollectionUri)
	PlanID         string `json:"PlanId"`         // The plan ID (system.PlanId)
	ProjectID      string `json:"ProjectId"`      // The project ID (system.TeamProjectId)
	HubName        string `json:"HubName"`        // The hub name (system.HostType)
	JobID          string `json:"JobId"`          // The job ID (system.JobId)
	TimelineID     string `json:"TimelineId"`     // The timeline ID (system.TimelineId)
	TaskInstanceID string `json:"TaskInstanceId"` // The task instance ID (system.TaskInstanceId)
	AuthToken      string `json:"AuthToken"`      // The job access token (system.AccessToken)
}

/*
ADOEventsURL generates an Azure DevOps API URL for the events endpoint.

See:

https://learn.microsoft.com/en-us/rest/api/azure/devops/distributedtask/events/post-event?view=azure-devops-rest-7.1&tabs=HTTP
*/
func (payload *ADOPayload) ADOEventsURL(instance string, apiVersion string) string {
	return fmt.Sprintf("https://%s/%s/_apis/distributedtask/hubs/%s/plans/%s/events?api-version=%s", instance, payload.ProjectID, payload.HubName, payload.PlanID, apiVersion)
}

/*
ADOConfig contains configuration values for connections to the Azure DevOps REST API.

See:

https://learn.microsoft.com/en-us/rest/api/azure/devops
*/
type ADOConfig struct {
	Instance     string // The ADO instance
	APIVersion   string // The ADO API version
	AuthUsername string // Prefix for the authentication token
}

/*
ReadFromEnv reads the following required environment variables
and populates the struct with the values:
  - ADO_DOMAIN: The ADO domain (default: dev.azure.com)
  - ADO_ORG: The ADO organization
  - ADO_API_VERSION: The ADO API version (default: 7.1-preview.3)
  - ADO_AUTH_USERNAME: Username for the 'basic auth' configuration, is ignored by the API
*/
func (config *ADOConfig) ReadFromEnv() {
	adoDomain := ReadEnvVarWithDefault("ADO_DOMAIN", "dev.azure.com")
	adoOrg := ReadRequiredEnvVar("ADO_ORG")
	config.Instance = fmt.Sprintf("%s/%s", adoDomain, adoOrg)
	config.APIVersion = ReadEnvVarWithDefault("ADO_API_VERSION", "7.1-preview.3")
	config.AuthUsername = ReadEnvVarWithDefault("ADO_AUTH_USERNAME", "ado-callback")
}

// // GetAuth generates the encoded token value for the 'Authorization: Basic <token>' header
// func (config *ADOConfig) GetAuth(token string) string {
// 	authStr := fmt.Sprintf("%s:%s", config.AuthUsername, token)
// 	return base64.StdEncoding.EncodeToString([]byte(authStr))
// }

/*
ADOCallbackConfig contains the configurations value
to generate a callback request to the Azure DevOps service connection.
*/
type ADOCallbackConfig struct {
	Config  *ADOConfig  // The ADO config
	Payload *ADOPayload // The ADO payload
	Result  string      // The reported outcome
}
