package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

// commitOutcomeUnknown means the database may or may not have committed.
// Callers must not delete an object that a committed row might reference.
type commitOutcomeUnknown struct {
	err error
}

func (e *commitOutcomeUnknown) Error() string {
	return "signature commit outcome is unknown: " + e.err.Error()
}

func (e *commitOutcomeUnknown) Unwrap() error { return e.err }

func UnknownCommitOutcome(err error) error {
	if err == nil {
		return nil
	}
	return &commitOutcomeUnknown{err: err}
}

func IsCommitOutcomeUnknown(err error) bool {
	var target *commitOutcomeUnknown
	return errors.As(err, &target)
}

// ClassifyCommitError treats a server ErrorResponse as a rejected commit.
// A lost response, timeout, or closed connection stays unknown: the row may exist.
func ClassifyCommitError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return err
	}
	return UnknownCommitOutcome(err)
}
