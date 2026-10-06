package repository

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestClassifyCommitError(t *testing.T) {
	if err := ClassifyCommitError(nil); err != nil {
		t.Fatal(err)
	}
	serverErr := &pgconn.PgError{Code: "40001", Message: "serialization failure"}
	if classified := ClassifyCommitError(serverErr); IsCommitOutcomeUnknown(classified) {
		t.Fatal("server rejection was treated as unknown")
	}
	unknown := ClassifyCommitError(fmt.Errorf("write failed: %w", errors.New("connection reset")))
	if !IsCommitOutcomeUnknown(unknown) {
		t.Fatal("lost commit response was treated as known")
	}
}
