package rebuild

import (
	"context"

	"github.com/google/uuid"
)

type RebuildRepository interface {
	CreateImportJob(ctx context.Context, manifest Manifest) error
	InsertStageBatch(ctx context.Context, rows []StageRow) error
	UpdateImportProgress(ctx context.Context, snapshotID uuid.UUID, importedRows int64) error
	MarkValidated(ctx context.Context, result ValidationResult) error
	MarkFailed(ctx context.Context, snapshotID uuid.UUID, code string) error
	GetJobStatus(ctx context.Context, snapshotID uuid.UUID) (JobStatus, error)
}

type ActivationRepository interface {
	Activate(ctx context.Context, snapshotID uuid.UUID) (ActivationResult, error)
	Rollback(ctx context.Context, snapshotID uuid.UUID) (RollbackResult, error)
	Cleanup(ctx context.Context, snapshotID uuid.UUID) (CleanupResult, error)
	GetRollbackEligibility(ctx context.Context, snapshotID uuid.UUID) (RollbackEligibility, error)
}
