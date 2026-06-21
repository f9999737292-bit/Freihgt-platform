package http

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"

	apperrors "github.com/freight-platform/api-gateway/internal/platform/errors"
	"github.com/freight-platform/api-gateway/internal/platform/respond"
)

const swaggerUIHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Freight Platform API Docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist/swagger-ui.css">
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist/swagger-ui-bundle.js"></script>
  <script>
    SwaggerUIBundle({
      url: "/openapi.json",
      dom_id: "#swagger-ui"
    });
  </script>
</body>
</html>
`

type OpenAPIHandler struct {
	dir string
}

func NewOpenAPIHandler(dir string) *OpenAPIHandler {
	return &OpenAPIHandler{dir: dir}
}

func (h *OpenAPIHandler) RegisterRoutes(r chi.Router) {
	r.Get("/docs", h.ServeDocs)
	r.Get("/docs/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/docs", http.StatusMovedPermanently)
	})
	r.Get("/openapi.yaml", h.ServeUnifiedYAML)
	r.Get("/openapi.json", h.ServeUnifiedJSON)
	r.Get("/openapi", h.ServeIndex)
	r.Get("/openapi/{file}", h.ServeFile)
}

func (h *OpenAPIHandler) ServeDocs(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(swaggerUIHTML))
}

func (h *OpenAPIHandler) ServeUnifiedYAML(w http.ResponseWriter, r *http.Request) {
	h.serveNamedFile(w, r, "openapi.yaml", "application/yaml")
}

func (h *OpenAPIHandler) ServeUnifiedJSON(w http.ResponseWriter, r *http.Request) {
	h.serveNamedFile(w, r, "openapi.json", "application/json")
}

func (h *OpenAPIHandler) ServeIndex(w http.ResponseWriter, _ *http.Request) {
	documents := []map[string]string{
		{"name": "Unified API", "format": "yaml", "url": "/openapi.yaml"},
		{"name": "Unified API", "format": "json", "url": "/openapi.json"},
		{"name": "Identity Service", "format": "yaml", "url": "/openapi/identity-service.yaml"},
		{"name": "Company Service", "format": "yaml", "url": "/openapi/company-service.yaml"},
		{"name": "Transport Order Service", "format": "yaml", "url": "/openapi/transport-order-service.yaml"},
		{"name": "RFx Service", "format": "yaml", "url": "/openapi/rfx-service.yaml"},
		{"name": "Shipment Service", "format": "yaml", "url": "/openapi/shipment-service.yaml"},
		{"name": "Document Service", "format": "yaml", "url": "/openapi/document-service.yaml"},
		{"name": "Billing Register Service", "format": "yaml", "url": "/openapi/billing-register-service.yaml"},
	}
	respond.JSON(w, http.StatusOK, map[string]any{"documents": documents})
}

func (h *OpenAPIHandler) ServeFile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "file")
	h.serveNamedFile(w, r, name, contentTypeFor(name))
}

func (h *OpenAPIHandler) serveNamedFile(w http.ResponseWriter, r *http.Request, name, contentType string) {
	path, err := h.safePath(name)
	if err != nil {
		respond.Error(w, apperrors.RouteNotFound("invalid openapi document path"))
		return
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			respond.Error(w, apperrors.RouteNotFound("openapi document not found"))
			return
		}
		respond.Error(w, apperrors.Internal("failed to read openapi document", err))
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *OpenAPIHandler) safePath(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, "/") || strings.Contains(name, `\`) {
		return "", errInvalidOpenAPIPath
	}

	ext := strings.ToLower(filepath.Ext(name))
	if ext != ".yaml" && ext != ".yml" && ext != ".json" {
		return "", errInvalidOpenAPIPath
	}

	cleanName := filepath.Clean(name)
	if cleanName != name || strings.HasPrefix(cleanName, "..") {
		return "", errInvalidOpenAPIPath
	}

	base, err := filepath.Abs(h.dir)
	if err != nil {
		return "", errInvalidOpenAPIPath
	}

	full, err := filepath.Abs(filepath.Join(base, cleanName))
	if err != nil {
		return "", errInvalidOpenAPIPath
	}

	rel, err := filepath.Rel(base, full)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", errInvalidOpenAPIPath
	}

	return full, nil
}

func contentTypeFor(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".json":
		return "application/json"
	default:
		return "application/yaml"
	}
}

var errInvalidOpenAPIPath = &invalidOpenAPIPathError{}

type invalidOpenAPIPathError struct{}

func (e *invalidOpenAPIPathError) Error() string {
	return "invalid openapi document path"
}
