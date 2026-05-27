// Package router provides the HTTP server setup and routing for the demo application.
package router

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/double12gzh/zap-demo/example/demo"
	"github.com/double12gzh/zap-demo/example/singleton"
	"github.com/double12gzh/zap-demo/logger"
	ginlogger "github.com/double12gzh/zap-demo/logger/middleware/gin"
)

// Response represents a generic API response
type Response struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Status  string      `json:"status"`
}

// ServHTTP starts the Gin HTTP server on port 8080.
func ServHTTP() {
	// Create a new Gin router with default middleware
	r := gin.Default()

	// Add RequestID middleware
	r.Use(ginlogger.RequestIDMiddleware())

	r.GET("/ping", func(c *gin.Context) {
		demo.Demo(c.Request.Context())

		singleton.CheckSingleton()

		c.JSON(http.StatusOK, Response{
			Message: "pong",
			Status:  "success",
		})
	})

	// Start the server on port 8080
	if err := r.Run(":8080"); err != nil {
		logger.Error(context.Background(), "HTTP server failed to start", zap.Error(err))
		panic(err)
	}
}
