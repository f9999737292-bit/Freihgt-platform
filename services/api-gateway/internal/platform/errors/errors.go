package errors

import "fmt"

type Code string

const (
	CodeRouteNotFound        Code = "ROUTE_NOT_FOUND"
	CodeUnauthorized         Code = "UNAUTHORIZED"
	CodeForbidden            Code = "FORBIDDEN"
	CodeServiceUnavailable   Code = "SERVICE_UNAVAILABLE"
	CodeInternal             Code = "INTERNAL_ERROR"
	CodeRateLimitExceeded    Code = "RATE_LIMIT_EXCEEDED"
	CodeRequestBodyTooLarge  Code = "REQUEST_BODY_TOO_LARGE"
)

type AppError struct {
	Code    Code
	Message string
	Details map[string]any
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

func RouteNotFound(message string) *AppError {
	return &AppError{Code: CodeRouteNotFound, Message: message, Details: map[string]any{}}
}

func Unauthorized(message string) *AppError {
	return &AppError{Code: CodeUnauthorized, Message: message, Details: map[string]any{}}
}

func Forbidden(message string) *AppError {
	return &AppError{Code: CodeForbidden, Message: message, Details: map[string]any{}}
}

func ServiceUnavailable(message string, service string) *AppError {
	return &AppError{
		Code:    CodeServiceUnavailable,
		Message: message,
		Details: map[string]any{"service": service},
	}
}

func Internal(message string, err error) *AppError {
	return &AppError{Code: CodeInternal, Message: message, Details: map[string]any{}, Err: err}
}

func RateLimitExceeded(message string) *AppError {
	return &AppError{Code: CodeRateLimitExceeded, Message: message, Details: map[string]any{}}
}

func RequestBodyTooLarge(message string) *AppError {
	return &AppError{Code: CodeRequestBodyTooLarge, Message: message, Details: map[string]any{}}
}
