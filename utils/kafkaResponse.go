package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"github.com/IBM/sarama"
	"github.com/pmas98/go-todo-service/models"
)

var (
	// Channel to pass the token verification response back to the caller
	tokenVerificationResultCh chan *models.TokenVerificationResponse
)

// Initialize Kafka consumer for token verification responses
func InitTokenVerificationConsumer(groupID string) error {
	config := sarama.NewConfig()
	config.Consumer.Group.Rebalance.Strategy = sarama.BalanceStrategyRoundRobin
	config.Consumer.Offsets.Initial = sarama.OffsetOldest

	consumerGroup, err := sarama.NewConsumerGroup(kafkaBrokers, groupID, config)
	if err != nil {
		return err
	}

	// Start consuming messages from the topic
	go func() {
		log.Println("Token verification consumer started")
		ctx := context.Background()
		handler := &TokenVerificationResponseHandler{}

		for {
			err := consumerGroup.Consume(ctx, []string{"token_verification_responses"}, handler)
			if err != nil {
				log.Printf("Error from token verification consumer: %v", err)
			}
		}
	}()

	return nil
}

// TokenVerificationResponseHandler handles token verification response messages
type TokenVerificationResponseHandler struct{}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (h *TokenVerificationResponseHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (h *TokenVerificationResponseHandler) Cleanup(sarama.ConsumerGroupSession) error {
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (h *TokenVerificationResponseHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for msg := range claim.Messages() {
		fmt.Printf("Token verification response received: value = %s, timestamp = %v, topic = %s\n", string(msg.Value), msg.Timestamp, msg.Topic)

		var response models.TokenVerificationResponse
		err := json.Unmarshal(msg.Value, &response)
		if err != nil {
			log.Printf("Error unmarshaling token verification response: %v", err)
			continue
		}

		// Send the response to the result channel
		if tokenVerificationResultCh != nil {
			tokenVerificationResultCh <- &response
		}

		session.MarkMessage(msg, "") // Mark message as processed
	}
	return nil
}

// SetTokenVerificationResultChannel sets the channel to receive token verification responses
func SetTokenVerificationResultChannel(ch chan *models.TokenVerificationResponse) {
	log.Println("Token verification result channel set")
	tokenVerificationResultCh = ch
}
