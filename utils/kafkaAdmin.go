package utils

import (
	"log"

	"github.com/IBM/sarama"
)

func InitKafkaAdmin() error {
	var err error
	config := sarama.NewConfig()
	adminClient, err = sarama.NewClusterAdmin(kafkaBrokers, config)
	if err != nil {
		return err
	}
	return nil
}

func CreateKafkaTopic(topicName string, numPartitions int32, replicationFactor int16) error {
	topicDetail := &sarama.TopicDetail{
		NumPartitions:     numPartitions,
		ReplicationFactor: replicationFactor,
	}

	err := adminClient.CreateTopic(topicName, topicDetail, false)
	if err != nil {
		log.Printf("Failed to create topic: %v", err)
		return err
	}
	return nil
}
