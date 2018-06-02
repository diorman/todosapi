package utils

import (
	"encoding/json"
	"fmt"

	"github.com/diorman/todospoc"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/endpoints"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

func LocalStackAWSConfig(endpointURL string) (*aws.Config, error) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS SDK config: %v", err)
	}
	cfg.Region = endpoints.UsEast1RegionID
	cfg.Credentials = aws.NewStaticCredentialsProvider("dummykey", "dummysecret", "dummytoken")
	cfg.EndpointResolver = aws.ResolveWithEndpointURL(endpointURL)
	return &cfg, nil
}

func QueueURL(queueName string) string {
	return fmt.Sprintf("%s/queue/%s", todospoc.Config.SQSEnpointURL, queueName)
}

type SQSClient interface {
	SendMessage(queueName string, messageBody interface{}) error
	CreateQueue(queueName string) (*sqs.CreateQueueOutput, error)
	ReceiveMessage(queueName string, maxMessages, waitTime int64) ([]sqs.Message, error)
	DeleteMessage(queueName string, receiptHandle string) error
}

type sqsClientImpl struct {
	*sqs.SQS
}

func NewSQSClient() (SQSClient, error) {
	cfg, err := LocalStackAWSConfig(todospoc.Config.SQSEnpointURL)
	if err != nil {
		return nil, fmt.Errorf("unable to create SQS client: %v", err)
	}
	return &sqsClientImpl{sqs.New(*cfg)}, nil
}

func (client *sqsClientImpl) SendMessage(queueName string, messageBody interface{}) error {
	messageBodyBytes, err := json.Marshal(messageBody)
	if err != nil {
		return fmt.Errorf("could not encode sqs message body: %v", err)
	}
	queueURL := QueueURL(queueName)
	messageBodyString := string(messageBodyBytes)
	req := client.SendMessageRequest(&sqs.SendMessageInput{
		QueueUrl:    &queueURL,
		MessageBody: &messageBodyString,
	})
	if _, err := req.Send(); err != nil {
		return fmt.Errorf("failed to send message to queue %s: %v", queueName, err)
	}
	return nil
}

func (client *sqsClientImpl) CreateQueue(queueName string) (*sqs.CreateQueueOutput, error) {
	req := client.SQS.CreateQueueRequest(&sqs.CreateQueueInput{QueueName: &queueName})
	return req.Send()
}

func (client *sqsClientImpl) ReceiveMessage(queueName string, maxMessages, waitTime int64) ([]sqs.Message, error) {
	var (
		queueURL = QueueURL(queueName)
	)
	req := client.SQS.ReceiveMessageRequest(&sqs.ReceiveMessageInput{
		QueueUrl:            &queueURL,
		MaxNumberOfMessages: &maxMessages,
		WaitTimeSeconds:     &waitTime,
	})
	res, err := req.Send()
	if err != nil {
		return nil, fmt.Errorf("failed to receive message from queue %s: %v", queueName, err)
	}
	return res.Messages, nil
}

func (client *sqsClientImpl) DeleteMessage(queueName string, receiptHandle string) error {
	var (
		queueURL = QueueURL(queueName)
	)
	req := client.SQS.DeleteMessageRequest(&sqs.DeleteMessageInput{
		QueueUrl:      &queueURL,
		ReceiptHandle: &receiptHandle,
	})
	if _, err := req.Send(); err != nil {
		return fmt.Errorf("failed to delete message from queue %s: %v", queueName, err)
	}
	return nil
}
