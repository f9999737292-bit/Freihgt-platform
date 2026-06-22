package errors

import "fmt"

type Code string

const (
	CodeValidation              Code = "VALIDATION_ERROR"
	CodeNotFound                Code = "NOT_FOUND"
	CodeInternal                Code = "INTERNAL_ERROR"
	CodeTenantRequired          Code = "TENANT_REQUIRED"
	CodeFormTemplateNotFound    Code = "FORM_TEMPLATE_NOT_FOUND"
	CodeEntityTypeInvalid       Code = "ENTITY_TYPE_INVALID"
	CodeEntityIDInvalid         Code = "ENTITY_ID_INVALID"
	CodeFormTemplateNotPublished Code = "FORM_TEMPLATE_NOT_PUBLISHED"
	CodeFormTemplateNotDraft     Code = "FORM_TEMPLATE_NOT_DRAFT"
	CodeFormTemplateConflict     Code = "FORM_TEMPLATE_CONFLICT"
	CodeFieldNotFound           Code = "FIELD_NOT_FOUND"
	CodeFieldInvalidType        Code = "FIELD_INVALID_TYPE"
	CodeValidationRuleFailed    Code = "VALIDATION_RULE_FAILED"
	CodeSystemFieldProtected    Code = "SYSTEM_FIELD_PROTECTED"
	CodeReadOnlyFieldProtected  Code = "READ_ONLY_FIELD_PROTECTED"
	CodeTenantMismatch          Code = "TENANT_MISMATCH"
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
		Message: "form template not found",
		Details: map[string]any{},
	}
}

func FormTemplateNotPublished() *AppError {
	return &AppError{
		Code:    CodeFormTemplateNotPublished,
		Message: "form template is not published",
		Details: map[string]any{},
	}
}

func FormTemplateNotDraft(status string) *AppError {
	return &AppError{
		Code:    CodeFormTemplateNotDraft,
		Message: "only draft form templates can be modified",
		Details: map[string]any{"status": status},
	}
}

func FormTemplateCloneSourceNotPublished(status string) *AppError {
	return &AppError{
		Code:    CodeValidation,
		Message: "only published templates can be cloned to draft",
		Details: map[string]any{"status": status, "field": "status"},
	}
}

func FormTemplateConflict(details map[string]any) *AppError {
	return &AppError{
		Code:    CodeFormTemplateConflict,
		Message: "form template already exists",
		Details: detailsOrEmpty(details),
	}
}

func EntityTypeInvalid(entityType string) *AppError {
	return &AppError{
		Code:    CodeEntityTypeInvalid,
		Message: "invalid entity_type",
		Details: map[string]any{"entity_type": entityType},
	}
}

func EntityIDInvalid(details map[string]any) *AppError {
	return &AppError{
		Code:    CodeEntityIDInvalid,
		Message: "invalid entity_id",
		Details: detailsOrEmpty(details),
	}
}

func FieldNotFound(fieldCode string) *AppError {
	return &AppError{
		Code:    CodeFieldNotFound,
		Message: "field not found in form template",
		Details: map[string]any{"field_code": fieldCode},
	}
}

func FieldInvalidType(fieldCode, fieldType string, details map[string]any) *AppError {
	d := detailsOrEmpty(details)
	d["field_code"] = fieldCode
	d["field_type"] = fieldType
	return &AppError{
		Code:    CodeFieldInvalidType,
		Message: "value does not match field type",
		Details: d,
	}
}

func ValidationRuleFailed(fieldCode string, details map[string]any) *AppError {
	d := detailsOrEmpty(details)
	d["field_code"] = fieldCode
	return &AppError{
		Code:    CodeValidationRuleFailed,
		Message: "validation rule failed",
		Details: d,
	}
}

func SystemFieldProtected(fieldCode string) *AppError {
	return &AppError{
		Code:    CodeSystemFieldProtected,
		Message: "system field cannot be modified",
		Details: map[string]any{"field_code": fieldCode},
	}
}

func ReadOnlyFieldProtected(fieldCode string) *AppError {
	return &AppError{
		Code:    CodeReadOnlyFieldProtected,
		Message: "read-only field cannot be modified",
		Details: map[string]any{"field_code": fieldCode},
	}
}

func TenantMismatch() *AppError {
	return &AppError{
		Code:    CodeTenantMismatch,
		Message: "tenant mismatch",
		Details: map[string]any{},
	}
}

func detailsOrEmpty(details map[string]any) map[string]any {
	if details == nil {
		return map[string]any{}
	}
	return details
}
