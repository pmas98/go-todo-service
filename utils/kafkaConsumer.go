package utils

import (
	"context"
	"fmt"
	"log"

	"github.com/IBM/sarama"
)

func InitKafkaConsumerGroup(groupID string) (sarama.ConsumerGroup, error) {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	consumerGroup, err := sarama.NewConsumerGroup(kafkaBrokers, groupID, config)
	if err != nil {
		return nil, err
	}
	return consumerGroup, nil
}

func ConsumeMessagesFromKafka(topic string, groupID string) error {
	consumerGroup, err := InitKafkaConsumerGroup(groupID)
	if err != nil {
		return err
	}

	ctx := context.Background()
	handler := &ConsumerGroupHandler{}

	for {
		err := consumerGroup.Consume(ctx, []string{topic}, handler)
		if err != nil {
			log.Printf("Error from consumer: %v", err)
			return err
		}
	}
}

// ConsumerGroupHandler represents a Sarama consumer group consumer
type ConsumerGroupHandler struct{}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *ConsumerGroupHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *ConsumerGroupHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (h *ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	log.Println("ConsumeClaim started")
	for msg := range claim.Messages() {
		fmt.Printf("Message received: value = %s, timestamp = %v, topic = %s\n", string(msg.Value), msg.Timestamp, msg.Topic)
		session.MarkMessage(msg, "")
	}
	log.Println("ConsumeClaim ended")
	return nil
}
