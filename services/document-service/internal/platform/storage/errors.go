package storage

import "errors"

// Stable storage failures. Error text is safe to log and must not carry
// credentials, object keys, or provider response bodies.
var (
	ErrObjectNotFound       = errors.New("OBJECT_NOT_FOUND")
	ErrObjectExists         = errors.New("OBJECT_UPLOAD_FAILED")
	ErrStorageUnavailable   = errors.New("OBJECT_STORAGE_UNAVAILABLE")
	ErrIntegrityMismatch    = errors.New("OBJECT_INTEGRITY_MISMATCH")
	ErrUploadFailed         = errors.New("OBJECT_UPLOAD_FAILED")
	ErrDownloadFailed       = errors.New("OBJECT_DOWNLOAD_FAILED")
	ErrInvalidStorageConfig = errors.New("OBJECT_STORAGE_UNAVAILABLE")
)
