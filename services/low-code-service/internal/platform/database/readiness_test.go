package database

import (
	"context"
	"testing"
)

func TestReadinessCheckerNilPool(t *testing.T) {
	checker := &ReadinessChecker{}
	result := checker.Check(context.Background())
	if result.Ready {
		t.Fatal("expected not ready for nil pool")
	}
	if result.Error == "" {
		t.Fatal("expected error message")
	}
}
