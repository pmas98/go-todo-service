package main

import (
	"log"
	"time"

	_ "github.com/pmas98/go-todo-service/docs"
	"github.com/pmas98/go-todo-service/routes"
	"github.com/pmas98/go-todo-service/utils"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Package main Product API documentation
//
// @title           Product API
// @version         0.1
// @description     This is a sample server for a Product API.
// @host            localhost:8081
// @BasePath        /api/v1
// @schemes         http https
// @consumes        application/json
// @produces        application/json
// @securityDefinitions.apikey Bearer
// @in              header
// @name            Authorization
// @description     Enter token in format Bearer <token>

var consumerReady = make(chan struct{})

func main() {
	go func() {
		for {
			err := utils.InitTokenVerificationConsumer("todo-service-consumer-group")
			if err != nil {
				log.Printf("Failed to initialize token verification consumer: %v. Retrying in 5 seconds...", err)
				time.Sleep(5 * time.Second)
				continue
			}
			close(consumerReady)
			return
		}
	}()
	<-consumerReady
	r := routes.SetupRouter()

	// Serve Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	r.Run(":8081")
}
