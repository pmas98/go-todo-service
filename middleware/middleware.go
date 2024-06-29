package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pmas98/go-todo-service/models"
	"github.com/pmas98/go-todo-service/utils"
)

func VerifyTokenAndInteractWithKafka(c *gin.Context) {
	tokenString := c.GetHeader("Authorization")
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header is required"})
		c.Abort()
		return
	}
	// Extract token from "Bearer <token>" format
	tokenString = tokenString[len("Bearer "):]

	// Create a channel for the result
	resultCh := make(chan *models.TokenVerificationResponse, 1)

	request := models.TokenVerificationRequest{
		Token: tokenString,
	}

	// Marshal the request to JSON
	requestJSON, err := json.Marshal(request)
	if err != nil {
		log.Printf("Error marshaling token verification request: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process token verification request"})
		c.Abort()
		return
	}

	// Send token to Kafka for verification
	errVer := utils.SendMessageJSONToKafka("token_verification_requests", requestJSON, "verify")
	if errVer != nil {
		log.Printf("Error sending token to Kafka: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send token for verification"})
		c.Abort()
		return
	}

	// Pass the result channel to ConsumerGroupHandler for receiving the response
	utils.SetTokenVerificationResultChannel(resultCh)

	// Wait for response from Kafka with a timeout
	select {
	case response := <-resultCh:
		if response.Valid {
			// Token is valid, proceed to next middleware or handler
			c.Set("userID", response.UserID) // Set userID in context for further use
			c.Next()
		} else {
			// Token is invalid
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
		}
	case <-time.After(10 * time.Second): // Timeout after 10 seconds
		log.Println("Timeout waiting for token verification response")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Timeout waiting for token verification response"})
		c.Abort()
	}
}
