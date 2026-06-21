package errors

import "fmt"

type Code string

const (
	CodeValidation Code = "VALIDATION_ERROR"
	CodeNotFound   Code = "NOT_FOUND"
	CodeConflict   Code = "CONFLICT"
	CodeInternal   Code = "INTERNAL_ERROR"
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

func (e *AppError) Unwrap() error {
	return e.Err
}

func Validation(message string, details map[string]any) *AppError {
	return &AppError{Code: CodeValidation, Message: message, Details: detailsOrEmpty(details)}
}

func NotFound(message string) *AppError {
	return &AppError{Code: CodeNotFound, Message: message, Details: map[string]any{}}
}

func Conflict(message string, details map[string]any) *AppError {
	return &AppError{Code: CodeConflict, Message: message, Details: detailsOrEmpty(details)}
}

func Internal(message string, err error) *AppError {
	return &AppError{Code: CodeInternal, Message: message, Details: map[string]any{}, Err: err}
}

func detailsOrEmpty(details map[string]any) map[string]any {
	if details == nil {
		return map[string]any{}
	}
	return details
}
