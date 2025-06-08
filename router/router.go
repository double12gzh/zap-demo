package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"zap-demo/common/middleware"
	"zap-demo/example/demo"
	"zap-demo/example/singleton"
	"zap-demo/logger"
)

// Response represents a generic API response
type Response struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Status  string      `json:"status"`
}

func ServHTTP() {
	go func() {
		for {
			_ = logger.GetLogger().Sync()
		}
	}()

	// Create a new Gin router with default middleware
	r := gin.Default()

	// Add RequestID middleware
	r.Use(func(c *gin.Context) {
		middleware.RequestIDMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c.Request = r
			c.Next()
		})).ServeHTTP(c.Writer, c.Request)
	})

	r.GET("/ping", func(c *gin.Context) {
		demo.Demo(c.Request.Context())

		singleton.CheckSingleton()

		c.JSON(http.StatusOK, Response{
			Message: "pong",
			Status:  "success",
		})
	})

	// Start the server on port 8080
	_ = r.Run(":8080")
}
