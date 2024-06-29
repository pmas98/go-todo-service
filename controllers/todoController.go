package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/jinzhu/gorm"
	"github.com/pmas98/go-todo-service/config"
	"github.com/pmas98/go-todo-service/models"
	"github.com/pmas98/go-todo-service/utils"
)

var db *gorm.DB
var rdb *redis.Client
var ctx context.Context

func init() {
	config.Connect()
	db = config.GetDB()
	rdb = config.GetRedis()
	ctx = config.GetContext()
	models.AutoMigrate(db)

	// Initialize Kafka producer
	err := utils.InitKafkaProducer()
	if err != nil {
		panic(err) // Handle error appropriately in your application startup
	}
}

// HealthCheck godoc
// @Summary      Health Check
// @Description  Get the health status of the service and its dependencies (database and Redis).
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      503  {object}  map[string]interface{}
// @Router       /health [get]
func HealthCheck(c *gin.Context) {
	// Check database connection
	dbErr := db.DB().Ping()

	kafkaErr := utils.SendMessageToKafka("test-topic", "Ping request!", "ping")

	// Check Redis connection
	redisErr := rdb.Ping(ctx).Err()

	// Prepare response
	response := gin.H{
		"status":    "up",
		"timestamp": time.Now().Format(time.RFC3339),
		"services": gin.H{
			"database": "up",
			"redis":    "up",
			"kafka":    "up",
		},
	}

	statusCode := http.StatusOK

	// Check for errors and update response accordingly
	if dbErr != nil {
		response["services"].(gin.H)["database"] = "down"
		response["status"] = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	if kafkaErr != nil {
		response["services"].(gin.H)["kafka"] = "down"
		response["status"] = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	if redisErr != nil {
		response["services"].(gin.H)["redis"] = "down"
		response["status"] = "degraded"
		statusCode = http.StatusServiceUnavailable
	}

	// Send response
	c.JSON(statusCode, response)
}

func CreateTopic(c *gin.Context) {
	type TopicInput struct {
		TopicName string `json:"topicName" binding:"required"`
	}
	var input TopicInput

	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	utils.InitKafkaAdmin()
	utils.CreateKafkaTopic(input.TopicName, 1, 1)

	c.JSON(http.StatusCreated, gin.H{"sucess": "New topic created"})
}

// GetGroups godoc
// @Summary      Retrieve all groups
// @Description  Get the list of all groups, including their associated ToDos. This endpoint first tries to fetch data from the Redis cache; if not available, it queries the database and caches the result.
// @Tags         groups
// @Produce      json
// @Success      200  {array}   models.Group
// @Failure      500  {object}  map[string]interface{}
// @Router       /groups [get]
func GetGroups(c *gin.Context) {
	var groups []models.Group

	// Try to get groups from Redis cache
	result, err := rdb.Get(ctx, "groups:all").Result()
	if err == redis.Nil {
		// If not in cache, query the database
		if err := db.Preload("ToDos").Find(&groups).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve groups"})
			return
		}

		// Cache the result
		data, err := json.Marshal(groups)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal groups"})
			return
		}
		rdb.Set(ctx, "groups:all", data, 5*time.Minute)
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis error: " + err.Error()})
		return
	} else {
		// If found in cache, unmarshal the data
		if err := json.Unmarshal([]byte(result), &groups); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unmarshal groups"})
			return
		}
	}

	c.JSON(http.StatusOK, groups)
}

// GetGroup godoc
// @Summary      Retrieve a group by ID
// @Description  Get details of a specific group, including its associated ToDos. This endpoint first tries to fetch data from the Redis cache; if not available, it queries the database and caches the result.
// @Tags         groups
// @Produce      json
// @Param        id    path      string                       true  "Group ID"
// @Success      200   {object}  models.Group
// @Failure      404   {object}  map[string]interface{}       "Group not found"
// @Failure      500   {object}  map[string]interface{}       "Internal Server Error"
// @Router       /groups/{id} [get]
func GetGroup(c *gin.Context) {
	id := c.Param("id")

	var group models.Group

	// Try to get the group from Redis cache
	result, err := rdb.Get(ctx, "group:"+id).Result()

	if err == nil {
		// Found in cache, unmarshal and return
		if err := json.Unmarshal([]byte(result), &group); err == nil {
			c.JSON(http.StatusOK, group)
			return
		}
		// If unmarshaling fails, we'll fall through to querying the database
	} else if err != redis.Nil {
		// An error occurred that wasn't just a cache miss
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis error: " + err.Error()})
		return
	}

	// Not found in cache or error occurred, query the database
	if err := db.Preload("ToDos").First(&group, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve group"})
		}
		return
	}

	// Cache the group for future requests
	groupJSON, err := json.Marshal(group)
	if err == nil {
		rdb.Set(ctx, "group:"+id, groupJSON, time.Hour) // Cache for 1 hour
	}

	c.JSON(http.StatusOK, group)
}

// CreateGroup godoc
// @Summary      Create a new group
// @Description  Create a new group with the provided JSON data.
// @Tags         groups
// @Accept       json
// @Produce      json
// @Param        group   body   models.Group   true   "Group object to be created"
// @Success      201     {object}        models.Group   "Created group"
// @Failure      400     {object}        map[string]interface{}   "Bad Request"
// @Router       /groups [post]
func CreateGroup(c *gin.Context) {
	var group models.Group
	if err := c.ShouldBindJSON(&group); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existingGroup models.Group
	if err := db.Where("name = ?", group.Name).First(&existingGroup).Error; err == nil {
		// If group with the same name exists, return a 409 Conflict error
		c.JSON(http.StatusConflict, gin.H{"error": "Group with this name already exists"})
		return
	} else if err != gorm.ErrRecordNotFound {
		// For other errors, return a 500 Internal Server Error
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	db.Create(&group)
	rdb.Del(ctx, "groups:all")
	c.JSON(http.StatusCreated, group)
}

// UpdateGroup godoc
// @Summary      Update a group by ID
// @Description  Update the name of a specific group identified by its ID.
// @Tags         groups
// @Accept       json
// @Produce      json
// @Param        id     path    string   true   "Group ID"
// @Param        name   body     string   true   "New name for the group"
// @Success      200     {object}  models.Group   "Updated group"
// @Failure      400     {object}  map[string]interface{}   "Bad Request"
// @Failure      404     {object}  map[string]interface{}   "Group not found"
// @Failure      500     {object}  map[string]interface{}   "Internal Server Error"
// @Router       /groups/{id} [put]
func UpdateGroup(c *gin.Context) {
	id := c.Param("id")

	// Estrutura para receber apenas o nome
	type UpdateInput struct {
		Name string `json:"name" binding:"required"`
	}

	var input UpdateInput

	// Vincula o JSON do corpo da requisição à estrutura input
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Busca o grupo existente
	var group models.Group
	if err := db.Where("id = ?", id).First(&group).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	// Atualiza apenas o nome
	if err := db.Model(&group).Update("name", input.Name).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update group"})
		return
	}

	// Invalida o cache
	rdb.Del(ctx, "groups:all")

	// Retorna o grupo atualizado
	c.JSON(http.StatusOK, group)
}

// DeleteGroup godoc
// @Summary      Delete a group by ID
// @Description  Delete a specific group identified by its ID.
// @Tags         groups
// @Produce      json
// @Param        id     path    string   true   "Group ID"
// @Success      200     {object}  string   "Group deleted"
// @Failure      404     {object}  map[string]interface{}   "Group not found"
// @Failure      500     {object}  map[string]interface{}   "Internal Server Error"
// @Router       /groups/{id} [delete]
func DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	var group models.Group

	// Finding the group
	err := db.Preload("ToDos").Where("id = ?", id).First(&group).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Group not found"})
		return
	}

	// Delete all ToDos associated with the group
	for _, todo := range group.ToDos {
		db.Delete(&todo)
	}

	// Now delete the group itself
	db.Delete(&group)

	// Optionally clear cache or other operations
	rdb.Del(ctx, "groups:all")
	rdb.Del(ctx, "todos:all")

	c.JSON(http.StatusOK, gin.H{"message": "Group and associated ToDos deleted"})
}

// GetToDos godoc
// @Summary      Retrieve all ToDos
// @Description  Get the list of all ToDos. This endpoint first tries to fetch data from the Redis cache; if not available, it queries the database and caches the result.
// @Tags         todos
// @Produce      json
// @Success      200     {array}   models.ToDo   "List of ToDos"
// @Failure      500     {object}  map[string]interface{}   "Internal Server Error"
// @Router       /todos [get]
func GetToDos(c *gin.Context) {
	var todos []models.ToDo

	// Try to get todos from Redis cache
	result, err := rdb.Get(ctx, "todos:all").Result()
	if err == redis.Nil {
		// If not in cache, query the database
		if err := db.Find(&todos).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve todos"})
			return
		}

		// Cache the result
		data, err := json.Marshal(todos)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal todos"})
			return
		}
		if err := rdb.Set(ctx, "todos:all", data, 5*time.Minute).Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cache todos"})
			return
		}
	} else if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis error: " + err.Error()})
		return
	} else {
		// If found in cache, unmarshal the data
		if err := json.Unmarshal([]byte(result), &todos); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unmarshal todos"})
			return
		}
	}

	c.JSON(http.StatusOK, todos)
}

// GetToDosById godoc
// @Summary      Retrieve a ToDo by ID
// @Description  Get details of a specific ToDo identified by its ID. This endpoint first tries to fetch data from the Redis cache; if not available, it queries the database and caches the result.
// @Tags         todos
// @Produce      json
// @Param        id     path    string   true   "ToDo ID"
// @Success      200     {object}  models.ToDo   "ToDo details"
// @Failure      404     {object}  map[string]interface{}   "ToDo not found"
// @Failure      500     {object}  map[string]interface{}   "Internal Server Error"
// @Router       /todos/{id} [get]
func GetToDosById(c *gin.Context) {
	id := c.Param("id")
	var todo models.ToDo
	result, err := rdb.Get(ctx, "todo:"+id).Result()
	if err == nil {
		// Found in cache, unmarshal and return
		if err := json.Unmarshal([]byte(result), &todo); err == nil {
			c.JSON(http.StatusOK, todo)
			return
		}
	} else if err != redis.Nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Redis error: " + err.Error()})
		return
	}

	// Not found in cache or error occurred, query the database
	if err := db.First(&todo, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ToDo not found"})
		return
	}

	// Cache the todo for future requests
	todoJSON, err := json.Marshal(todo)
	if err == nil {
		if err := rdb.Set(ctx, "todo:"+id, todoJSON, time.Hour).Err(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to cache todo"})
			return
		}
	}

	c.JSON(http.StatusOK, todo)
}

// GetToDosByDate godoc
// @Summary      Retrieve ToDos by due date
// @Description  Get a list of ToDos that have a due date matching the specified date.
// @Tags         todos
// @Produce      json
// @Param        date   path   string   true   "Due date (format: YYYY-MM-DD)"
// @Success      200     {array}  models.ToDo   "List of ToDos"
// @Failure      500     {object}  map[string]interface{}   "Internal Server Error"
// @Router       /todos/date/{date} [get]
func GetToDosByDate(c *gin.Context) {
	date := c.Param("date")
	var todos []models.ToDo
	db.Where("date(due_date) = ?", date).Find(&todos)
	c.JSON(http.StatusOK, todos)
}

// CreateToDo godoc
// @Summary      Create a new ToDo
// @Description  Create a new ToDo with the provided JSON data.
// @Tags         todos
// @Accept       json
// @Produce      json
// @Param        todo   body   models.ToDo   true   "ToDo object to be created"
// @Success      201     {object}  models.ToDo   "Created ToDo"
// @Failure      400     {object}  map[string]interface{}   "Bad Request"
// @Router       /todos [post]
func CreateToDo(c *gin.Context) {
	var todo models.ToDo

	// Bind the JSON request body to the ToDo struct
	if err := c.ShouldBindJSON(&todo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if GroupID exists
	var group models.Group
	if err := db.First(&group, todo.GroupID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "GroupID does not exist"})
		return
	}

	// Create the ToDo in the database
	db.Create(&todo)

	// Clear the cache
	rdb.Del(ctx, "todos:all")

	// Return the created ToDo with status 201 Created
	c.JSON(http.StatusCreated, todo)
}

// UpdateToDo godoc
// @Summary      Update a ToDo by ID
// @Description  Update details of a specific ToDo identified by its ID.
// @Tags         todos
// @Accept       json
// @Produce      json
// @Param        id     path    string   true   "ToDo ID"
// @Param        todo   body     models.ToDo   true   "Updated ToDo object"
// @Success      200     {object}  models.ToDo   "Updated ToDo"
// @Failure      400     {object}  map[string]interface{}   "Bad Request"
// @Failure      404     {object}  map[string]interface{}   "ToDo not found"
// @Router       /todos/{id} [put]
func UpdateToDo(c *gin.Context) {
	id := c.Param("id")
	var todo models.ToDo

	if err := db.Where("id = ?", id).First(&todo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ToDo not found"})
		return
	}
	if err := c.ShouldBindJSON(&todo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	db.Save(&todo)
	rdb.Del(ctx, "todos:all")

	c.JSON(http.StatusOK, todo)
}

// DeleteToDo godoc
// @Summary      Delete a ToDo by ID
// @Description  Delete a specific ToDo identified by its ID.
// @Tags         todos
// @Produce      json
// @Param        id     path    string   true   "ToDo ID"
// @Success      200     {object}  string   "ToDo deleted"
// @Failure      404     {object}  map[string]interface{}   "ToDo not found"
// @Router       /todos/{id} [delete]
func DeleteToDo(c *gin.Context) {
	id := c.Param("id")
	var todo models.ToDo
	if err := db.Where("id = ?", id).First(&todo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "ToDo not found"})
		return
	}
	db.Delete(&todo)
	rdb.Del(ctx, "todos:all")
	c.JSON(http.StatusOK, gin.H{"message": "ToDo deleted"})
}
