package rebuild

import (
	"context"

	"github.com/google/uuid"
)

type RebuildRepository interface {
	CreateImportJob(ctx context.Context, manifest Manifest) error
	InsertStageBatch(ctx context.Context, rows []StageRow) error
	MarkValidated(ctx context.Context, result ValidationResult) error
	MarkFailed(ctx context.Context, snapshotID uuid.UUID, code string) error
	GetJobStatus(ctx context.Context, snapshotID uuid.UUID) (JobStatus, error)
}
