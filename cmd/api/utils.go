package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type JSONResponse struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

func (app *application) errorJSON(c *gin.Context, err error, status ...int) {
	statusCode := http.StatusBadRequest
	if len(status) > 0 {
		statusCode = status[0]
	}

	c.AbortWithStatusJSON(statusCode, JSONResponse{
		Error:   true,
		Message: err.Error(),
	})
}
