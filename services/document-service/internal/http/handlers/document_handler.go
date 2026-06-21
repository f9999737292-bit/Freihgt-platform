package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/freight-platform/document-service/internal/domain"
	apperrors "github.com/freight-platform/document-service/internal/platform/errors"
	"github.com/freight-platform/document-service/internal/platform/respond"
	"github.com/freight-platform/document-service/internal/repository"
	"github.com/freight-platform/document-service/internal/service"
)

type DocumentHandler struct {
	service *service.DocumentService
}

func NewDocumentHandler(svc *service.DocumentService) *DocumentHandler {
	return &DocumentHandler{service: svc}
}

func (h *DocumentHandler) Health(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "document-service",
	})
}

type createDocumentRequest struct {
	TenantID          string          `json:"tenant_id"`
	DocumentNumber    string          `json:"document_number"`
	DocumentType      string          `json:"document_type"`
	OwnerCompanyID    string          `json:"owner_company_id"`
	RelatedEntityType *string         `json:"related_entity_type"`
	RelatedEntityID   *string         `json:"related_entity_id"`
	LegalLanguage     string          `json:"legal_language"`
	PayloadJSON       json.RawMessage `json:"payload_json"`
}

type createVersionRequest struct {
	TenantID       string          `json:"tenant_id"`
	PayloadJSON    json.RawMessage `json:"payload_json"`
	PayloadXMLPath *string         `json:"payload_xml_path"`
	PDFFilePath    *string         `json:"pdf_file_path"`
}

type createFileRequest struct {
	TenantID          string  `json:"tenant_id"`
	DocumentVersionID string  `json:"document_version_id"`
	FileType          string  `json:"file_type"`
	StorageProvider   string  `json:"storage_provider"`
	BucketName        *string `json:"bucket_name"`
	ObjectKey         string  `json:"object_key"`
	FileName          *string `json:"file_name"`
	MimeType          *string `json:"mime_type"`
	FileSizeBytes     *int64  `json:"file_size_bytes"`
	ChecksumSHA256    *string `json:"checksum_sha256"`
}

type tenantRequest struct {
	TenantID string `json:"tenant_id"`
}

type cancelDocumentRequest struct {
	TenantID string `json:"tenant_id"`
	Reason   string `json:"reason"`
}

func (h *DocumentHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	input, err := parseCreateDocumentRequest(req)
	if err != nil {
		respond.Error(w, err)
		return
	}
	doc, err := h.service.Create(r.Context(), input)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toDocumentResponse(doc))
}

func (h *DocumentHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	detail, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toDocumentDetailResponse(detail))
}

func (h *DocumentHandler) List(w http.ResponseWriter, r *http.Request) {
	tenantID, err := domain.ParseUUID(r.URL.Query().Get("tenant_id"), "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	filter := domain.ListDocumentsFilter{
		TenantID: tenantID,
		Limit:    parseLimit(r),
		Offset:   parseOffset(r),
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("document_type")); raw != "" {
		filter.DocumentType = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("document_status")); raw != "" {
		filter.DocumentStatus = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("related_entity_type")); raw != "" {
		filter.RelatedEntityType = &raw
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("related_entity_id")); raw != "" {
		id, err := domain.ParseUUID(raw, "related_entity_id")
		if err != nil {
			respond.Error(w, err)
			return
		}
		filter.RelatedEntityID = &id
	}

	docs, total, err := h.service.List(r.Context(), filter)
	if err != nil {
		respond.Error(w, err)
		return
	}
	items := make([]map[string]any, 0, len(docs))
	for i := range docs {
		items = append(items, toDocumentResponse(&docs[i]))
	}
	respond.JSON(w, http.StatusOK, map[string]any{"items": items, "total": total})
}

func (h *DocumentHandler) CreateVersion(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createVersionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	version, err := h.service.CreateVersion(r.Context(), id, domain.CreateDocumentVersionInput{
		TenantID: tenantID, PayloadJSON: req.PayloadJSON,
		PayloadXMLPath: req.PayloadXMLPath, PDFFilePath: req.PDFFilePath,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toDocumentVersionResponse(version))
}

func (h *DocumentHandler) AddFile(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req createFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	versionID, err := domain.ParseUUID(req.DocumentVersionID, "document_version_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	file, err := h.service.AddFile(r.Context(), id, domain.CreateDocumentFileInput{
		TenantID: tenantID, DocumentVersionID: versionID, FileType: req.FileType,
		StorageProvider: req.StorageProvider, BucketName: req.BucketName, ObjectKey: req.ObjectKey,
		FileName: req.FileName, MimeType: req.MimeType, FileSizeBytes: req.FileSizeBytes,
		ChecksumSHA256: req.ChecksumSHA256,
	})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toDocumentFileResponse(file))
}

func (h *DocumentHandler) ReadyForSigning(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req tenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	doc, err := h.service.ReadyForSigning(r.Context(), id, domain.ReadyForSigningInput{TenantID: tenantID})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toDocumentResponse(doc))
}

func (h *DocumentHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req cancelDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	doc, err := h.service.Cancel(r.Context(), id, domain.CancelDocumentInput{TenantID: tenantID, Reason: req.Reason})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toDocumentResponse(doc))
}

func (h *DocumentHandler) Archive(w http.ResponseWriter, r *http.Request) {
	id, err := domain.ParseUUID(chi.URLParam(r, "id"), "id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	var req tenantRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respond.Error(w, apperrors.Validation("invalid JSON body", map[string]any{"field": "body"}))
		return
	}
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		respond.Error(w, err)
		return
	}
	doc, err := h.service.Archive(r.Context(), id, domain.ArchiveDocumentInput{TenantID: tenantID})
	if err != nil {
		respond.Error(w, err)
		return
	}
	respond.JSON(w, http.StatusOK, toDocumentResponse(doc))
}

func parseCreateDocumentRequest(req createDocumentRequest) (domain.CreateDocumentInput, error) {
	tenantID, err := domain.ParseUUID(req.TenantID, "tenant_id")
	if err != nil {
		return domain.CreateDocumentInput{}, err
	}
	ownerCompanyID, err := domain.ParseUUID(req.OwnerCompanyID, "owner_company_id")
	if err != nil {
		return domain.CreateDocumentInput{}, err
	}
	relatedEntityID, err := domain.ParseOptionalUUID(derefString(req.RelatedEntityID), "related_entity_id")
	if err != nil {
		return domain.CreateDocumentInput{}, err
	}
	var relatedEntityType *string
	if req.RelatedEntityType != nil && strings.TrimSpace(*req.RelatedEntityType) != "" {
		value := strings.TrimSpace(*req.RelatedEntityType)
		relatedEntityType = &value
	}
	return domain.CreateDocumentInput{
		TenantID:          tenantID,
		DocumentNumber:    req.DocumentNumber,
		DocumentType:      req.DocumentType,
		OwnerCompanyID:    ownerCompanyID,
		RelatedEntityType: relatedEntityType,
		RelatedEntityID:   relatedEntityID,
		LegalLanguage:     req.LegalLanguage,
		PayloadJSON:       req.PayloadJSON,
	}, nil
}

func toDocumentResponse(d *domain.Document) map[string]any {
	return map[string]any{
		"id":                  d.ID.String(),
		"tenant_id":           d.TenantID.String(),
		"document_number":     d.DocumentNumber,
		"document_type":       d.DocumentType,
		"document_status":     d.DocumentStatus,
		"owner_company_id":    d.OwnerCompanyID.String(),
		"related_entity_type": d.RelatedEntityType,
		"related_entity_id":   optionalUUIDString(d.RelatedEntityID),
		"legal_language":      d.LegalLanguage,
		"created_at":          d.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at":          d.UpdatedAt.UTC().Format(time.RFC3339),
		"version":             d.Version,
	}
}

func toDocumentDetailResponse(detail *repository.DocumentDetail) map[string]any {
	resp := toDocumentResponse(detail.Document)
	resp["latest_version"] = nil
	if detail.LatestVersion != nil {
		resp["latest_version"] = toDocumentVersionResponse(detail.LatestVersion)
	}
	files := make([]map[string]any, 0, len(detail.Files))
	for i := range detail.Files {
		files = append(files, toDocumentFileResponse(&detail.Files[i]))
	}
	resp["files"] = files
	return resp
}

func toDocumentVersionResponse(v *domain.DocumentVersion) map[string]any {
	var payload any
	if len(v.PayloadJSON) > 0 {
		_ = json.Unmarshal(v.PayloadJSON, &payload)
	}
	return map[string]any{
		"id":               v.ID.String(),
		"document_id":      v.DocumentID.String(),
		"version_number":   v.VersionNumber,
		"payload_json":     payload,
		"payload_xml_path": v.PayloadXMLPath,
		"pdf_file_path":    v.PDFFilePath,
		"created_at":       v.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func toDocumentFileResponse(f *domain.DocumentFile) map[string]any {
	return map[string]any{
		"id":                  f.ID.String(),
		"document_id":         f.DocumentID.String(),
		"document_version_id": optionalUUIDString(f.DocumentVersionID),
		"file_type":           f.FileType,
		"storage_provider":    f.StorageProvider,
		"bucket_name":         f.BucketName,
		"object_key":          f.ObjectKey,
		"file_name":           f.FileName,
		"mime_type":           f.MimeType,
		"file_size_bytes":     f.FileSizeBytes,
		"checksum_sha256":     f.ChecksumSHA256,
		"created_at":          f.CreatedAt.UTC().Format(time.RFC3339),
	}
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
