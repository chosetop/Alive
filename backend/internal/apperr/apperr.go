// Package apperr defines the single application error type used across all
// layers of the backend.
//
// Rule: layers below the HTTP boundary must not know that HTTP exists. A
// service returns apperr.NotFound(...) and the HTTP layer decides that this
// means 404. The same service logic can then back a CLI command or a gRPC
// server without change.
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a stable, machine-readable error identifier. Clients may branch on
// it, so values must not change once released.
type Code string

const (
	CodeInternal     Code = "INTERNAL"
	CodeInvalidInput Code = "INVALID_INPUT"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeNotFound     Code = "NOT_FOUND"
	CodeConflict     Code = "CONFLICT"
	CodeUnavailable  Code = "UNAVAILABLE"

	// CodeInvalidCredentials reports a rejected login.
	//
	// Separate from CodeUnauthorized, which reports a request that arrived without
	// a usable session. Both answer 401, but a client needs to tell them apart:
	// the first means "the password was wrong", the second means "log in again",
	// and admin shows different things for each.
	CodeInvalidCredentials Code = "INVALID_CREDENTIALS"

	// CodeRateLimited reports that a client has made too many attempts.
	CodeRateLimited Code = "RATE_LIMITED"
)

// Error carries everything the HTTP layer needs to build a response, plus a
// cause that is written to logs only.
//
// The cause never reaches the response body. A leaked driver error hands table
// and column names to an attacker.
type Error struct {
	Code    Code
	Message string
	Status  int
	// Fields carries per-field validation detail. Nil for non-validation errors.
	Fields map[string]string

	cause error
}

func (e *Error) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap exposes the cause to errors.Is and errors.As.
func (e *Error) Unwrap() error { return e.cause }

// WithCause attaches an internal cause for logging and returns the receiver.
func (e *Error) WithCause(cause error) *Error {
	e.cause = cause
	return e
}

// WithField records a validation failure for a single input field.
func (e *Error) WithField(name, reason string) *Error {
	if e.Fields == nil {
		e.Fields = make(map[string]string)
	}
	e.Fields[name] = reason
	return e
}

func newError(code Code, status int, message string) *Error {
	return &Error{Code: code, Status: status, Message: message}
}

// Internal reports a server-side failure. The message is deliberately generic;
// put the real detail in the cause.
func Internal(message string) *Error {
	return newError(CodeInternal, http.StatusInternalServerError, message)
}

func InvalidInput(message string) *Error {
	return newError(CodeInvalidInput, http.StatusBadRequest, message)
}

func Unauthorized(message string) *Error {
	return newError(CodeUnauthorized, http.StatusUnauthorized, message)
}

func Forbidden(message string) *Error {
	return newError(CodeForbidden, http.StatusForbidden, message)
}

func NotFound(message string) *Error {
	return newError(CodeNotFound, http.StatusNotFound, message)
}

func Conflict(message string) *Error {
	return newError(CodeConflict, http.StatusConflict, message)
}

// Unavailable reports that a dependency the request needs is not reachable.
func Unavailable(message string) *Error {
	return newError(CodeUnavailable, http.StatusServiceUnavailable, message)
}

// InvalidCredentials reports a rejected login. 401, not 400: the request was
// well formed, it was the credentials that were refused.
func InvalidCredentials(message string) *Error {
	return newError(CodeInvalidCredentials, http.StatusUnauthorized, message)
}

// RateLimited reports that a client has made too many attempts.
func RateLimited(message string) *Error {
	return newError(CodeRateLimited, http.StatusTooManyRequests, message)
}

// From maps any error onto an *Error. Errors that are already *Error pass
// through unchanged; anything else becomes an opaque internal error that keeps
// the original as its cause.
func From(err error) *Error {
	if err == nil {
		return nil
	}

	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr
	}

	return Internal("unexpected internal error").WithCause(err)
}
