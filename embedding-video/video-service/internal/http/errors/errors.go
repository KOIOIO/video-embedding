package errors

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"nlp-video-analysis/internal/http/dto"
)

type APIError struct {
	Status  int
	Code    string
	Message string
}

func InvalidArgument(msg string) *APIError {
	return &APIError{
		Status:  http.StatusBadRequest,
		Code:    "invalid_argument",
		Message: msg,
	}
}

func NotFound(code, msg string) *APIError {
	if code == "" {
		code = "not_found"
	}
	return &APIError{
		Status:  http.StatusNotFound,
		Code:    code,
		Message: msg,
	}
}

func Internal(msg string) *APIError {
	if msg == "" {
		msg = "internal server error"
	}
	return &APIError{
		Status:  http.StatusInternalServerError,
		Code:    "internal",
		Message: msg,
	}
}

func Write(c *gin.Context, err *APIError) {
	if err == nil {
		err = Internal("")
	}
	if err.Status == 0 {
		err.Status = http.StatusInternalServerError
	}
	if err.Code == "" {
		err.Code = "internal"
	}
	if err.Message == "" {
		err.Message = "internal server error"
	}
	c.JSON(err.Status, dto.ErrorResponse{
		Success: false,
		Error: dto.ErrorBody{
			Code:    err.Code,
			Message: err.Message,
		},
	})
	c.Abort()
}
