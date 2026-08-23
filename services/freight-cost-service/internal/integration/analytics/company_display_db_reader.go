//go:build integration

package analytics

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/freight-platform/freight-cost-service/internal/provider"
)

type dbCompanyDisplayReader struct {
	pool *pgxpool.Pool
}

func newDBCompanyDisplayReader(pool *pgxpool.Pool) provider.CompanyDisplayReader {
	return &dbCompanyDisplayReader{pool: pool}
}

func (r *dbCompanyDisplayReader) BatchGetCompanyDisplay(
	ctx context.Context,
	tenantID uuid.UUID,
	companyIDs []uuid.UUID,
) (map[uuid.UUID]provider.CompanyDisplay, error) {
	result := make(map[uuid.UUID]provider.CompanyDisplay)
	if len(companyIDs) == 0 {
		return result, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, legal_name, short_name, status
		FROM core.companies
		WHERE tenant_id = $1 AND id = ANY($2) AND deleted_at IS NULL`, tenantID, companyIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var item provider.CompanyDisplay
		if err := rows.Scan(&item.CompanyID, &item.LegalName, &item.ShortName, &item.Status); err != nil {
			return nil, err
		}
		result[item.CompanyID] = item
	}
	return result, rows.Err()
}
