package handlers

import (
	"net/http"
	"strconv"
	"time"
)

func parseLimit(r *http.Request) int {
	limit := 20
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return limit
		}
		limit = parsed
	}
	return limit
}

func parseOffset(r *http.Request) int {
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return offset
		}
		offset = parsed
	}
	return offset
}

func formatDate(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format("2006-01-02")
}

func formatDateTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC().Format("2006-01-02T15:04:05Z")
}
