package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"time"

	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
)

type Event events.SQSEvent

var (
	cfg       *aws.Config
	taskCfg   *ECSTaskConfig
	adoCfg    *ADOConfig
	ecsClient *ecs.Client
)

func init() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	taskCfg = new(ECSTaskConfig)
	taskCfg.ReadFromEnv()

	adoCfg.ReadFromEnv()
	adoCfg = new(ADOConfig)

	ctx := context.TODO()
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		slog.Error("unable to load AWS configuration", slog.Any("err", err))
		os.Exit(1)
	}

	ecsClient = ecs.NewFromConfig(cfg)
}

func handler(ctx context.Context, event Event) error {
	for _, record := range event.Records {

		var payload *ADOPayload
		err := json.Unmarshal([]byte(record.Body), &payload)
		if err != nil {
			slog.Error("failed to parse message body", slog.Any("err", err))
			return err
		}

		taskCfg.SetClientToken(payload.AuthToken)

		result, err := RunFargateTask(ctx, ecsClient, taskCfg)
		if err != nil {
			slog.Error("failed to run task", slog.Any("err", err))
			return err
		}

		slog.Info("run task", slog.Any("res", result))

		taskARN := aws.ToString(result.Tasks[0].TaskArn)

		runTaskOutcome := "failed"
		for {
			taskStatus, err := GetTaskLastStatus(ctx, ecsClient, &ECSTaskReadConfig{
				Cluster: taskCfg.Cluster,
				TaskARN: taskARN,
			})
			if err != nil {
				slog.Error("failed to get task status", slog.Any("err", err))
				return err
			}

			if taskStatus == "RUNNING" {
				runTaskOutcome = "succeeded"
				break
			} else if taskStatus == "STOPPED" {
				break
			} else {
				time.Sleep(1 * time.Second)
			}
		}

		callbackResponse, err := ADOCallback(&http.Client{}, &ADOCallbackConfig{
			Config:  adoCfg,
			Payload: payload,
			Result:  runTaskOutcome,
		})
		if err != nil {
			slog.Error("failed to send ADO callback", slog.Any("err", err))
			return err
		}

		slog.Info("ADO response", slog.Any("res", callbackResponse))
	}

	return nil
}

func main() {
	lambda.StartWithOptions(handler, lambda.WithEnableSIGTERM())
}
