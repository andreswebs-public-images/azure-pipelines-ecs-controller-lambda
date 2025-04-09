package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// RunFargateTask invokes the AWS ECS RunTask API with a pre-defined configuration.
func RunFargateTask(ctx context.Context, client *ecs.Client, config *ECSTaskConfig) (*ecs.RunTaskOutput, error) {
	return client.RunTask(ctx, &ecs.RunTaskInput{
		Cluster:              aws.String(config.Cluster),
		TaskDefinition:       aws.String(config.TaskDefinition),
		Count:                aws.Int32(1),
		LaunchType:           types.LaunchTypeFargate,
		PropagateTags:        types.PropagateTagsTaskDefinition,
		EnableECSManagedTags: *aws.Bool(true),
		EnableExecuteCommand: *aws.Bool(true),
		ClientToken:          aws.String(config.ClientToken),
		NetworkConfiguration: &types.NetworkConfiguration{
			AwsvpcConfiguration: &types.AwsVpcConfiguration{
				Subnets:        config.Subnets,
				SecurityGroups: config.SecurityGroups,
				AssignPublicIp: types.AssignPublicIpEnabled,
			},
		},
	})
}

// GetTaskLastStatus returns an AWS ECS task's last status
func GetTaskLastStatus(ctx context.Context, client *ecs.Client, config *ECSTaskReadConfig) (status string, err error) {
	result, err := client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
		Cluster: aws.String(config.Cluster),
		Tasks:   []string{config.TaskARN},
	})
	if err != nil {
		return
	}

	if len(result.Tasks) > 0 {
		status = aws.ToString(result.Tasks[0].LastStatus)
	} else {
		err = fmt.Errorf("failed to describe task %s", config.TaskARN)
	}

	return
}

/*
GenerateClientToken creates a hash of the input string, encodes it to base64,
and converts it into a string that includes up to 64 ASCII characters.
This token can be used as a Client Token for idempotent requests to the AWS ECS RunTask API.

See:

https://docs.aws.amazon.com/AmazonECS/latest/APIReference/ECS_Idempotency.html#RunTaskIdempotency
*/
func GenerateClientToken(input string) string {
	hash := sha256.Sum256([]byte(input))
	encoded := base64.StdEncoding.EncodeToString(hash[:])
	if len(encoded) > 64 {
		encoded = encoded[:64]
	}
	return encoded
}

/*
ReadRequiredEnvVar reads a specified environment variable and returns the value,
or exits with status 1 if the value is unset or empty.
*/
func ReadRequiredEnvVar(name string) string {
	value := os.Getenv(name)
	if value == "" {
		slog.Error(fmt.Sprintf("missing required environment variable %s", name))
		os.Exit(1)
	}
	return value
}

/*
ReadEnvVarWithDefault reads a specified environment variable and returns the value,
or returns a specified default value if the value is unset or empty.
*/
func ReadEnvVarWithDefault(name string, defaultVal string) string {
	value := os.Getenv(name)
	if value == "" {
		return defaultVal
	}
	return value
}

/*
ADOCallback calls back to the Azure DevOps service connection with the process outcome.

See:

https://learn.microsoft.com/en-us/azure/devops/pipelines/process/invoke-checks?view=azure-devops
*/
func ADOCallback(client *http.Client, config *ADOCallbackConfig) (data string, err error) {
	token := config.Config.GetAuth(config.Payload.AuthToken)

	headers := map[string]string{
		"Accept":        "application/json",
		"Authorization": fmt.Sprintf("Bearer %s", token),
	}

	body := map[string]string{
		"name":   "TaskCompleted",
		"jobId":  config.Payload.JobID,
		"taskId": config.Payload.TaskInstanceID,
		"result": config.Result,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		err = fmt.Errorf("failed to marshal JSON body: %w", err)
		return
	}

	url := config.Payload.ADOEventsURL(config.Config.Instance, config.Config.APIVersion)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(bodyBytes))
	if err != nil {
		err = fmt.Errorf("failed to create HTTP request: %w", err)
		return
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	res, err := client.Do(req)
	if err != nil {
		err = fmt.Errorf("failed to execute HTTP request: %w", err)
		return
	}

	resBytes, err := readResponse(res)
	if err != nil {
		return
	}

	data = string(resBytes)
	return
}

func readResponse(res *http.Response) (data []byte, err error) {
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 399 {
		err = fmt.Errorf("unexpected status code: %d", res.StatusCode)
		return
	}

	data, err = io.ReadAll(res.Body)
	if err != nil {
		err = fmt.Errorf("failed to read response body: %w", err)
		return
	}

	return
}
