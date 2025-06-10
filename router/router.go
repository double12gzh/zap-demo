package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/double12gzh/zap-demo/example/demo"
	"github.com/double12gzh/zap-demo/example/singleton"
	"github.com/double12gzh/zap-demo/logger"
	"github.com/double12gzh/zap-demo/router/middleware"
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
	r.Use(middleware.RequestIDMiddleware())

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
