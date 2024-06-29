package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/pmas98/go-todo-service/controllers"
	"github.com/pmas98/go-todo-service/middleware"
)

func SetupRouter() *gin.Engine {
	r := gin.Default()
	r.Use(middleware.VerifyTokenAndInteractWithKafka)
	api := r.Group("/api/v1")
	{
		api.GET("/health", controllers.HealthCheck)
		api.POST("/createTopic", controllers.CreateTopic)

		api.GET("/todos/:id", controllers.GetToDosById)
		api.GET("/todos/date/:date", controllers.GetToDosByDate)
		api.GET("/todos", controllers.GetToDos)
		api.POST("/todos", controllers.CreateToDo)
		api.PUT("/todos/:id", controllers.UpdateToDo)
		api.DELETE("/todos/:id", controllers.DeleteToDo)

		api.POST("/groups", controllers.CreateGroup)
		api.GET("/groups", controllers.GetGroups)
		api.GET("/groups/:id", controllers.GetGroup)
		api.DELETE("/groups/:id", controllers.DeleteGroup)
	}

	return r
}
