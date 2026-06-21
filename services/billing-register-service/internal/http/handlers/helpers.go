package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
)

func parseLimit(r *http.Request) int {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			limit = parsed
		}
	}
	return limit
}

func parseOffset(r *http.Request) int {
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			offset = parsed
		}
	}
	return offset
}

func optionalUUIDString(id *uuid.UUID) any {
	if id == nil {
		return nil
	}
	return id.String()
}

func formatDateTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format("2006-01-02T15:04:05Z")
}
