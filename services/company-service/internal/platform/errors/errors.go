package errors

import "fmt"

type Code string

const (
	CodeValidation   Code = "VALIDATION_ERROR"
	CodeNotFound     Code = "NOT_FOUND"
	CodeConflict     Code = "CONFLICT"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeInternal     Code = "INTERNAL_ERROR"

	CodeAuthenticationRequired             Code = "AUTHENTICATION_REQUIRED"
	CodeTenantContextConflict              Code = "TENANT_CONTEXT_CONFLICT"
	CodeRelationshipCommandsDisabled       Code = "RELATIONSHIP_COMMANDS_DISABLED"
	CodeRelationshipRequestForbidden       Code = "RELATIONSHIP_REQUEST_FORBIDDEN"
	CodeRelationshipAcceptanceForbidden    Code = "RELATIONSHIP_ACCEPTANCE_FORBIDDEN"
	CodeRelationshipRequestNotFound        Code = "RELATIONSHIP_REQUEST_NOT_FOUND"
	CodeRelationshipRequestExpired         Code = "RELATIONSHIP_REQUEST_EXPIRED"
	CodeRelationshipRequestCancelled       Code = "RELATIONSHIP_REQUEST_CANCELLED"
	CodeRelationshipRequestAlreadyAccepted Code = "RELATIONSHIP_REQUEST_ALREADY_ACCEPTED"
	CodeRelationshipRequestStaleSource     Code = "RELATIONSHIP_REQUEST_STALE_SOURCE"
	CodeRelationshipRequestPendingConflict Code = "RELATIONSHIP_REQUEST_PENDING_CONFLICT"
	CodeRelationshipIdempotencyConflict    Code = "RELATIONSHIP_IDEMPOTENCY_CONFLICT"
	CodeRelationshipSourceUnavailable      Code = "RELATIONSHIP_SOURCE_UNAVAILABLE"
	CodeRelationshipStoreUnavailable       Code = "RELATIONSHIP_STORE_UNAVAILABLE"
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

func Unauthorized(message string) *AppError {
	return &AppError{Code: CodeUnauthorized, Message: message, Details: map[string]any{}}
}

func Forbidden(message string) *AppError {
	return &AppError{Code: CodeForbidden, Message: message, Details: map[string]any{}}
}

func New(code Code, message string) *AppError {
	return &AppError{Code: code, Message: message, Details: map[string]any{}}
}

func StatusOf(code Code) int {
	switch code {
	case CodeValidation:
		return 400
	case CodeAuthenticationRequired, CodeUnauthorized:
		return 401
	case CodeRelationshipCommandsDisabled, CodeRelationshipRequestForbidden, CodeRelationshipAcceptanceForbidden, CodeForbidden:
		return 403
	case CodeNotFound, CodeRelationshipRequestNotFound:
		return 404
	case CodeConflict, CodeTenantContextConflict, CodeRelationshipRequestExpired, CodeRelationshipRequestCancelled,
		CodeRelationshipRequestAlreadyAccepted, CodeRelationshipRequestStaleSource, CodeRelationshipRequestPendingConflict,
		CodeRelationshipIdempotencyConflict:
		return 409
	case CodeRelationshipSourceUnavailable, CodeRelationshipStoreUnavailable:
		return 503
	default:
		return 500
	}
}

func detailsOrEmpty(details map[string]any) map[string]any {
	if details == nil {
		return map[string]any{}
	}
	return details
}
