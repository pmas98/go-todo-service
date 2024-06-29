package utils

import (
	"log"
	"os"

	"github.com/IBM/sarama"
)

var (
	kafkaBrokers = []string{os.Getenv("Kafka_URL")} // Replace with your Kafka broker(s) address
	producer     sarama.SyncProducer
	adminClient  sarama.ClusterAdmin
)

func InitKafkaProducer() error {
	// Configure the Kafka producer
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true

	// Create a new sync producer
	var err error
	producer, err = sarama.NewSyncProducer(kafkaBrokers, config)
	if err != nil {
		return err
	}
	return nil
}

func SendMessageToKafka(topic string, message string, key string) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(message),
		Key:   sarama.StringEncoder(key),
	}

	// Send message to Kafka
	_, _, err := producer.SendMessage(msg)
	if err != nil {
		log.Printf("Failed to send message to Kafka: %v", err)
		return err
	}
	return nil
}

func SendMessageJSONToKafka(topic string, message []byte, key string) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.StringEncoder(message),
		Key:   sarama.StringEncoder(key),
	}

	// Send message to Kafka
	_, _, err := producer.SendMessage(msg)
	if err != nil {
		log.Printf("Failed to send message to Kafka: %v", err)
		return err
	}
	return nil
}
