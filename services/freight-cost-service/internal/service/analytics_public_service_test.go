package service

import (
	"net/url"
	"testing"
)

func TestParseAnalyticsPublicQueryDefaults(t *testing.T) {
	q, err := ParseAnalyticsPublicQuery(url.Values{}, map[string]string{"spend_total": "spend_total"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if q.Limit != analyticsDefaultLimit {
		t.Fatalf("expected default limit %d got %d", analyticsDefaultLimit, q.Limit)
	}
	if q.DateDimension != analyticsDateDimension {
		t.Fatalf("expected date dimension %s got %s", analyticsDateDimension, q.DateDimension)
	}
}

func TestParseAnalyticsPublicQueryInvalidSort(t *testing.T) {
	values := url.Values{}
	values.Set("sort", "unknown_field")
	_, err := ParseAnalyticsPublicQuery(values, map[string]string{"spend_total": "spend_total"})
	if err == nil {
		t.Fatal("expected validation error for unknown sort")
	}
}

func TestParseAnalyticsPublicQueryInvalidRange(t *testing.T) {
	values := url.Values{}
	values.Set("from", "2026-12-01")
	values.Set("to", "2026-01-01")
	_, err := ParseAnalyticsPublicQuery(values, nil)
	if err == nil {
		t.Fatal("expected validation error when from > to")
	}
}
