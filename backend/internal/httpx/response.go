// Package httpx holds the HTTP response envelope shared by every handler.
//
// Every response body is a JSON object with exactly one top-level key:
// "data" on success, "error" on failure. Clients can therefore tell the two
// apart without inspecting the status code, and adding a sibling key later
// (for example "meta" for pagination) does not break existing parsers.
package httpx

import (
	"github.com/gin-gonic/gin"

	"github.com/p30huiwei/alive/backend/internal/apperr"
)

// successBody is the envelope for a successful response.
type successBody struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

// errorBody is the envelope for a failed response.
type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    apperr.Code       `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
	// RequestID lets a user quote one value that finds the matching log line.
	RequestID string `json:"request_id,omitempty"`
}

// OK writes 200 with the payload wrapped in "data".
func OK(c *gin.Context, data any) {
	c.JSON(200, successBody{Data: data})
}

// Created writes 201 with the payload wrapped in "data".
func Created(c *gin.Context, data any) {
	c.JSON(201, successBody{Data: data})
}

// List writes 200 with the payload in "data" and pagination detail in "meta".
func List(c *gin.Context, data any, meta any) {
	c.JSON(200, successBody{Data: data, Meta: meta})
}

// NoContent writes 204 with an empty body.
func NoContent(c *gin.Context) {
	c.Status(204)
}

// Error converts err to the response envelope and writes it with the status
// carried by the error. The error's cause is not part of the output; it reaches
// the log through the error middleware instead.
func Error(c *gin.Context, err error) {
	appErr := apperr.From(err)

	c.AbortWithStatusJSON(appErr.Status, errorBody{
		Error: errorDetail{
			Code:      appErr.Code,
			Message:   appErr.Message,
			Fields:    appErr.Fields,
			RequestID: RequestIDFromContext(c.Request.Context()),
		},
	})
}
