package storage

import "strings"

// ValidateObjectKey rejects caller-shaped paths. Keys are server-owned.
func ValidateObjectKey(key string) error {
	if key == "" || strings.Contains(key, "\x00") || strings.Contains(key, "\\") || strings.Contains(key, ":") {
		return ErrUploadFailed
	}
	if strings.HasPrefix(key, "/") || strings.Contains(key, "..") || strings.Contains(key, "//") {
		return ErrUploadFailed
	}
	return nil
}
