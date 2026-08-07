package rebuild

import "time"

func JobStatusToResponse(job JobStatus) StatusResponse {
	resp := StatusResponse{
		State:            job.State,
		Scope:            job.Scope,
		ExpectedRows:     job.ExpectedRows,
		ImportedRows:     job.ImportedRows,
		TenantCount:      job.TenantCount,
		ActivatedRows:    job.ActivatedRows,
		BackupRows:       job.BackupRows,
		RollbackEligible: job.RollbackEligible,
		ChecksumMatched:  job.ChecksumMatched,
		StartedAt:        job.StartedAt.UTC().Format(time.RFC3339),
		ErrorCode:        job.ErrorCode,
	}
	if job.ValidatedAt != nil {
		formatted := job.ValidatedAt.UTC().Format(time.RFC3339)
		resp.ValidatedAt = &formatted
	}
	if job.ActivatedAt != nil {
		formatted := job.ActivatedAt.UTC().Format(time.RFC3339)
		resp.ActivatedAt = &formatted
	}
	if job.RolledBackAt != nil {
		formatted := job.RolledBackAt.UTC().Format(time.RFC3339)
		resp.RolledBackAt = &formatted
	}
	return resp
}
