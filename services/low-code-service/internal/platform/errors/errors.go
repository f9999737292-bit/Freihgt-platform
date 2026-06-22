package errors

import "fmt"

type Code string

const (
	CodeValidation           Code = "VALIDATION_ERROR"
	CodeNotFound             Code = "NOT_FOUND"
	CodeInternal             Code = "INTERNAL_ERROR"
	CodeTenantRequired       Code = "TENANT_REQUIRED"
	CodeFormTemplateNotFound Code = "FORM_TEMPLATE_NOT_FOUND"
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

func Internal(message string, err error) *AppError {
	return &AppError{Code: CodeInternal, Message: message, Details: map[string]any{}, Err: err}
}

func TenantRequired() *AppError {
	return &AppError{
		Code:    CodeTenantRequired,
		Message: "tenant id is required",
		Details: map[string]any{"header": "X-Tenant-ID"},
	}
}

func FormTemplateNotFound() *AppError {
	return &AppError{
		Code:    CodeFormTemplateNotFound,
		Message: "published form template not found",
		Details: map[string]any{},
	}
}

func detailsOrEmpty(details map[string]any) map[string]any {
	if details == nil {
		return map[string]any{}
	}
	return details
}
