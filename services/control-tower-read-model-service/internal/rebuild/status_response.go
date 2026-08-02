package rebuild

import "time"

func JobStatusToResponse(job JobStatus) StatusResponse {
	resp := StatusResponse{
		State:           job.State,
		Scope:           job.Scope,
		ExpectedRows:    job.ExpectedRows,
		ImportedRows:    job.ImportedRows,
		TenantCount:     job.TenantCount,
		ChecksumMatched: job.ChecksumMatched,
		StartedAt:       job.StartedAt.UTC().Format(time.RFC3339),
		ErrorCode:       job.ErrorCode,
	}
	if job.ValidatedAt != nil {
		formatted := job.ValidatedAt.UTC().Format(time.RFC3339)
		resp.ValidatedAt = &formatted
	}
	return resp
}
