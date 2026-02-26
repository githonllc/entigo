package ginx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Response is the standard JSON response envelope.
type Response struct {
	OK        bool   `json:"ok"`
	Message   string `json:"message,omitempty"`
	ErrorCode int    `json:"error_code,omitempty"`
	Data      any    `json:"data,omitempty"`
}

// SendOK sends a JSON success response with HTTP 200 OK status.
// Additional key-value pairs can be provided via args (must be string key followed by value).
//
// Example:
//
//	SendOK(c, "Success", users, "total", 100, "page", 1)
//
// Produces:
//
//	{"ok": true, "message": "Success", "data": users, "total": 100, "page": 1}
func SendOK(c *gin.Context, message string, data any, args ...any) {
	response := gin.H{
		"ok":      true,
		"message": message,
		"data":    data,
	}

	for len(args) >= 2 {
		if key, ok := args[0].(string); ok {
			response[key] = args[1]
		}
		args = args[2:]
	}

	if response["message"] == "" || response["message"] == nil {
		delete(response, "message")
	}

	c.JSON(http.StatusOK, response)
}

// SendError sends a JSON error response with the given HTTP status, error code, and message.
func SendError(c *gin.Context, httpStatus int, errorCode int, errorMsg string) {
	c.AbortWithStatusJSON(httpStatus, Response{
		OK:        false,
		Message:   errorMsg,
		ErrorCode: errorCode,
	})
}

// WriteError translates an error into an appropriate HTTP error response.
// It checks for gin.Error, gorm.ErrRecordNotFound, and CustomError types
// to determine the correct HTTP status and error code. Unrecognized errors
// result in a 500 Internal Server Error.
func WriteError(c *gin.Context, err error) {
	var ginErr *gin.Error
	if errors.As(err, &ginErr) {
		err = ginErr.Err
	}

	customErr := &CustomError{}
	if IsErrNotFound(err) {
		SendError(c, http.StatusNotFound, ErrCodeNotFound, err.Error())
	} else if errors.As(err, &customErr) {
		SendError(c, customErr.HTTPStatus, customErr.ErrorCode, customErr.Message)
	} else {
		slog.Warn("unhandled error", slog.Any("error", err))
		SendError(c, http.StatusInternalServerError, ErrCodeInternalServerError, err.Error())
	}
}
