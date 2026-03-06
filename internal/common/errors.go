package common

import (
	"fmt"
)

// OpenStackError represents a standard OpenStack API error
type OpenStackError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *OpenStackError) Error() string {
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Common error constructors
func NewUnauthorizedError(message string) *OpenStackError {
	return &OpenStackError{
		StatusCode: 401,
		Code:       "unauthorized",
		Message:    message,
	}
}

func NewForbiddenError(message string) *OpenStackError {
	return &OpenStackError{
		StatusCode: 403,
		Code:       "forbidden",
		Message:    message,
	}
}

func NewNotFoundError(resource string) *OpenStackError {
	return &OpenStackError{
		StatusCode: 404,
		Code:       "itemNotFound",
		Message:    fmt.Sprintf("%s not found", resource),
	}
}

func NewConflictError(message string) *OpenStackError {
	return &OpenStackError{
		StatusCode: 409,
		Code:       "conflict",
		Message:    message,
	}
}

func NewBadRequestError(message string) *OpenStackError {
	return &OpenStackError{
		StatusCode: 400,
		Code:       "badRequest",
		Message:    message,
	}
}

func NewServiceUnavailableError(message string) *OpenStackError {
	return &OpenStackError{
		StatusCode: 503,
		Code:       "serviceUnavailable",
		Message:    message,
	}
}
