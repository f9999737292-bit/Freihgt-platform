package storage

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type userMetaKey struct{}

// WithUserMetadata attaches non-authoritative object metadata. The database remains canonical.
func WithUserMetadata(ctx context.Context, meta map[string]string) context.Context {
	clean := make(map[string]string, len(meta))
	for key, value := range meta {
		key = strings.TrimSpace(strings.ToLower(key))
		value = strings.TrimSpace(value)
		switch key {
		case "attachment_id", "document_id", "tenant_id", "sha256", "media_type":
			if value != "" && !strings.ContainsAny(value, "\r\n") {
				clean[key] = value
			}
		}
	}
	return context.WithValue(ctx, userMetaKey{}, clean)
}

type S3ObjectStore struct {
	endpoint  *url.URL
	region    string
	bucket    string
	accessKey string
	secretKey string
	pathStyle bool
	prefix    string
	client    *http.Client
	now       func() time.Time
}

func NewS3ObjectStore(cfg Config) (*S3ObjectStore, error) {
	if cfg.Provider != "s3" {
		return nil, ErrInvalidStorageConfig
	}
	if err := validateEndpoint(cfg.Endpoint, cfg.TLS); err != nil {
		return nil, err
	}
	parsed, err := url.Parse(cfg.Endpoint)
	if err != nil {
		return nil, ErrInvalidStorageConfig
	}
	return &S3ObjectStore{
		endpoint:  parsed,
		region:    cfg.Region,
		bucket:    cfg.Bucket,
		accessKey: cfg.AccessKey,
		secretKey: cfg.SecretKey,
		pathStyle: cfg.PathStyle,
		prefix:    cfg.Prefix,
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		now: time.Now,
	}, nil
}

func (s *S3ObjectStore) Put(ctx context.Context, objectKey string, reader io.Reader, maxBytes int64) (int64, error) {
	started := time.Now()
	body, err := readLimited(reader, maxBytes)
	if err != nil {
		observe("put", started, err)
		return 0, err
	}
	key, err := s.resolveKey(objectKey)
	if err != nil {
		observe("put", started, err)
		return 0, err
	}
	headers := map[string]string{"Content-Type": "application/octet-stream", "If-None-Match": "*"}
	if meta, _ := ctx.Value(userMetaKey{}).(map[string]string); meta != nil {
		for name, value := range meta {
			headers["x-amz-meta-"+name] = value
		}
	}
	resp, err := s.do(ctx, http.MethodPut, key, body, headers)
	if err != nil {
		observe("put", started, err)
		return 0, err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusPreconditionFailed || resp.StatusCode == http.StatusConflict {
		observe("put", started, ErrObjectExists)
		return 0, ErrObjectExists
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		observe("put", started, ErrUploadFailed)
		return 0, ErrUploadFailed
	}
	observe("put", started, nil)
	return int64(len(body)), nil
}

func (s *S3ObjectStore) Get(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	started := time.Now()
	key, err := s.resolveKey(objectKey)
	if err != nil {
		observe("get", started, err)
		return nil, err
	}
	resp, err := s.do(ctx, http.MethodGet, key, nil, nil)
	if err != nil {
		observe("get", started, err)
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		observe("get", started, ErrObjectNotFound)
		return nil, ErrObjectNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		observe("get", started, ErrDownloadFailed)
		return nil, ErrDownloadFailed
	}
	observe("get", started, nil)
	return resp.Body, nil
}

func (s *S3ObjectStore) Exists(ctx context.Context, objectKey string) (bool, error) {
	started := time.Now()
	status, err := s.head(ctx, objectKey)
	if err != nil {
		observe("head", started, err)
		return false, err
	}
	if status == http.StatusNotFound {
		observe("head", started, nil)
		return false, nil
	}
	if status < 200 || status >= 300 {
		observe("head", started, ErrStorageUnavailable)
		return false, ErrStorageUnavailable
	}
	observe("head", started, nil)
	return true, nil
}

func (s *S3ObjectStore) Metadata(ctx context.Context, objectKey string) (ObjectMetadata, error) {
	started := time.Now()
	key, err := s.resolveKey(objectKey)
	if err != nil {
		observe("head", started, err)
		return ObjectMetadata{}, err
	}
	resp, err := s.do(ctx, http.MethodHead, key, nil, nil)
	if err != nil {
		observe("head", started, err)
		return ObjectMetadata{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		observe("head", started, ErrObjectNotFound)
		return ObjectMetadata{}, ErrObjectNotFound
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		observe("head", started, ErrStorageUnavailable)
		return ObjectMetadata{}, ErrStorageUnavailable
	}
	observe("head", started, nil)
	return ObjectMetadata{SizeBytes: resp.ContentLength}, nil
}

func (s *S3ObjectStore) Delete(ctx context.Context, objectKey string) error {
	started := time.Now()
	key, err := s.resolveKey(objectKey)
	if err != nil {
		observe("delete", started, err)
		return err
	}
	resp, err := s.do(ctx, http.MethodDelete, key, nil, nil)
	if err != nil {
		observe("delete", started, err)
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode == http.StatusNotFound || (resp.StatusCode >= 200 && resp.StatusCode < 300) {
		observe("delete", started, nil)
		return nil
	}
	observe("delete", started, ErrUploadFailed)
	return ErrUploadFailed
}

func (s *S3ObjectStore) head(ctx context.Context, objectKey string) (int, error) {
	key, err := s.resolveKey(objectKey)
	if err != nil {
		return 0, err
	}
	resp, err := s.do(ctx, http.MethodHead, key, nil, nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}

func (s *S3ObjectStore) resolveKey(objectKey string) (string, error) {
	if err := ValidateObjectKey(objectKey); err != nil {
		return "", err
	}
	if s.prefix == "" {
		return objectKey, nil
	}
	return s.prefix + "/" + objectKey, nil
}

func (s *S3ObjectStore) objectURL(key string) string {
	base := *s.endpoint
	escaped := escapeKey(key)
	if s.pathStyle {
		base.Path = strings.TrimRight(base.Path, "/") + "/" + s.bucket + "/" + escaped
		base.RawPath = ""
		return base.String()
	}
	base.Host = s.bucket + "." + base.Host
	base.Path = "/" + escaped
	return base.String()
}

func escapeKey(key string) string {
	parts := strings.Split(key, "/")
	for i, part := range parts {
		parts[i] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func (s *S3ObjectStore) do(ctx context.Context, method, key string, body []byte, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, s.objectURL(key), bytes.NewReader(body))
	if err != nil {
		return nil, ErrStorageUnavailable
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	signRequest(req, sha256Hex(body), s.region, s.accessKey, s.secretKey, s.now())
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, ErrStorageUnavailable
	}
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		return nil, ErrStorageUnavailable
	}
	return resp, nil
}

func readLimited(reader io.Reader, maxBytes int64) ([]byte, error) {
	if reader == nil {
		return nil, ErrUploadFailed
	}
	body, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, ErrUploadFailed
	}
	if int64(len(body)) > maxBytes {
		return nil, ErrUploadFailed
	}
	return body, nil
}
