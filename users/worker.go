package users

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/diorman/todospoc/utils"
)

type Worker struct {
	sqs       utils.SQSClient
	store     Store
	kong      KongClient
	queueName string
}

func NewWorker(sqs utils.SQSClient, store Store, kong KongClient, queueName string) *Worker {
	return &Worker{
		sqs:       sqs,
		store:     store,
		kong:      kong,
		queueName: queueName,
	}
}

func (w *Worker) Start() {
	for {
		msgs, err := w.sqs.ReceiveMessage(w.queueName, 10, 5)
		if err != nil {
			log.Printf("failed to read message: %v\n", err)
			continue
		}
		if len(msgs) > 0 {
			log.Printf("recived %d messages\n", len(msgs))
		}
		for _, msg := range msgs {
			userID, err := decodeUserCreatedMessage(msg)
			if err != nil {
				log.Println(err.Error())
				continue
			}
			if err := w.setupKongConsumer(userID); err != nil {
				log.Println(err.Error())
				continue
			}
			if err := w.sqs.DeleteMessage(w.queueName, *msg.ReceiptHandle); err != nil {
				log.Println(err.Error())
				continue
			}
			log.Printf("message processed %s\n", *msg.Body)
		}
	}
}

func decodeUserCreatedMessage(message sqs.Message) (string, error) {
	decoder := json.NewDecoder(bytes.NewBuffer([]byte(*message.Body)))
	messageBody := struct {
		EventType string `json:"event_type"`
		Payload   struct {
			UserID string `json:"user_id"`
		} `json:"payload"`
	}{}
	if err := decoder.Decode(&messageBody); err != nil {
		return "", fmt.Errorf("could not decode sqs message: %v", err)
	}
	if messageBody.EventType != "user_created" {
		return "", fmt.Errorf("unexpected message: %s", messageBody)
	}
	return messageBody.Payload.UserID, nil
}

func (w *Worker) setupKongConsumer(userID string) error {
	consumerID, err := w.kong.createConsumer(userID)
	if err != nil {
		return err
	}
	if err := w.kong.createJWTCredentials(consumerID); err != nil {
		return err
	}

	return w.store.setConsumerID(userID, consumerID)
}
