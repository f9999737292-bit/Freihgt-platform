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

func TestFC22G_ParseAnalyticsPublicQuerySortInjection(t *testing.T) {
	allowlist := map[string]string{"spend_total": "spend_total", "lane_label": "lane_label"}
	for _, sort := range []string{"foo;drop table", "1 desc", "' OR 1=1", "spend_total;delete"} {
		values := url.Values{}
		values.Set("sort", sort)
		if _, err := ParseAnalyticsPublicQuery(values, allowlist); err == nil {
			t.Fatalf("expected 400-class validation for sort=%q", sort)
		}
	}
	values := url.Values{}
	values.Set("sort", "-spend_total")
	q, err := ParseAnalyticsPublicQuery(values, allowlist)
	if err != nil {
		t.Fatalf("valid descending sort: %v", err)
	}
	if !q.SortDesc || q.Sort != "spend_total" {
		t.Fatalf("expected descending spend_total, got sort=%q desc=%v", q.Sort, q.SortDesc)
	}
}

func TestFC22G_ParseAnalyticsPublicQueryPaginationAbuse(t *testing.T) {
	for _, limit := range []string{"0", "-1"} {
		values := url.Values{}
		values.Set("limit", limit)
		if _, err := ParseAnalyticsPublicQuery(values, nil); err == nil {
			t.Fatalf("expected validation error for limit=%s", limit)
		}
	}
	values := url.Values{}
	values.Set("limit", "100")
	q, err := ParseAnalyticsPublicQuery(values, nil)
	if err != nil {
		t.Fatalf("max limit should pass: %v", err)
	}
	if q.Limit != analyticsMaxLimit {
		t.Fatalf("expected max limit %d got %d", analyticsMaxLimit, q.Limit)
	}
	values = url.Values{}
	values.Set("limit", "1000000")
	q, err = ParseAnalyticsPublicQuery(values, nil)
	if err != nil {
		t.Fatalf("over-max limit should cap: %v", err)
	}
	if q.Limit != analyticsMaxLimit {
		t.Fatalf("expected capped limit %d got %d", analyticsMaxLimit, q.Limit)
	}
	values = url.Values{}
	values.Set("offset", "-5")
	if _, err := ParseAnalyticsPublicQuery(values, nil); err == nil {
		t.Fatal("expected validation error for negative offset")
	}
}

func TestFC22G_ParseAnalyticsPublicQueryInvalidCurrency(t *testing.T) {
	values := url.Values{}
	values.Set("currency", "NOTISO")
	if _, err := ParseAnalyticsPublicQuery(values, nil); err == nil {
		t.Fatal("expected validation error for invalid currency")
	}
}
