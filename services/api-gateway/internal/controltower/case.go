package controltower

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/freight-platform/api-gateway/internal/controltowerreadmodel"
)

func (s *Service) ListCases(ctx context.Context, reqCtx RequestContext, r *http.Request) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	q := r.URL.Query()
	filter := controltowerreadmodel.CasesFilter{
		Status: q.Get("status"), Severity: q.Get("severity"),
		OwnerUserID: q.Get("ownerUserId"), ShipmentID: q.Get("shipmentId"),
		Search: q.Get("search"), Preset: q.Get("preset"),
		Page: parseIntQuery(q.Get("page"), 1), Limit: parseIntQuery(q.Get("limit"), 50),
		MyCases: q.Get("myCases") == "true", Unassigned: q.Get("unassigned") == "true",
		IncludeClosed: q.Get("includeClosed") == "true",
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ListCases(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, filter)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return payload, nil
}

func (s *Service) GetCase(ctx context.Context, reqCtx RequestContext, caseID string) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.GetCase(rmCtx, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, caseID)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return enrichCasePayload(ctx, s, reqCtx, payload)
}

func (s *Service) ProxyCaseMutation(ctx context.Context, reqCtx RequestContext, method, caseID, suffix string, raw []byte) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	path := controltowerreadmodel.CasePath(caseID, suffix)
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ProxyCaseJSON(rmCtx, method, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, path, raw)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	if len(payload) == 0 {
		return nil, nil
	}
	return enrichCasePayload(ctx, s, reqCtx, payload)
}

func (s *Service) ProxyCaseCreate(ctx context.Context, reqCtx RequestContext, raw []byte) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ProxyCaseJSON(rmCtx, http.MethodPost, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, "/internal/v1/control-tower/cases", raw)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return enrichCasePayload(ctx, s, reqCtx, payload)
}

func (s *Service) GetCaseKPI(ctx context.Context, reqCtx RequestContext) (json.RawMessage, error) {
	if err := s.requireReadModel(); err != nil {
		return nil, err
	}
	rmCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.readModelCfg.Timeout)
	defer cancel()
	payload, depErr := s.readModel.ProxyCaseJSON(rmCtx, http.MethodGet, reqCtx.TenantID, reqCtx.UserID, reqCtx.RequestID, controltowerreadmodel.CaseKPIPath, nil)
	if depErr != nil {
		return nil, mapWorkspaceDependencyError(depErr)
	}
	return payload, nil
}

func enrichCasePayload(ctx context.Context, s *Service, reqCtx RequestContext, payload json.RawMessage) (json.RawMessage, error) {
	userNames, _ := s.loadTenantUserNames(ctx, reqCtx)
	var doc map[string]any
	if err := json.Unmarshal(payload, &doc); err != nil {
		return payload, nil
	}
	if ownerID, ok := doc["ownerUserId"].(string); ok {
		if name, ok := userNames[ownerID]; ok {
			doc["ownerDisplayName"] = name
		}
	}
	if createdBy, ok := doc["createdByUserId"].(string); ok {
		if name, ok := userNames[createdBy]; ok {
			doc["createdByDisplayName"] = name
		}
	}
	out, err := json.Marshal(doc)
	if err != nil {
		return payload, nil
	}
	return out, nil
}

func parseIntQuery(raw string, fallback int) int {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	var n int
	if _, err := fmt.Sscanf(raw, "%d", &n); err != nil || n < 1 {
		return fallback
	}
	return n
}
