package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type ObjectMetadata struct {
	SizeBytes int64
}

type ObjectStore interface {
	Put(ctx context.Context, objectKey string, reader io.Reader, maxBytes int64) (int64, error)
	Get(ctx context.Context, objectKey string) (io.ReadCloser, error)
	Exists(ctx context.Context, objectKey string) (bool, error)
	Metadata(ctx context.Context, objectKey string) (ObjectMetadata, error)
	Delete(ctx context.Context, objectKey string) error
}

type LocalObjectStore struct {
	root string
}

func NewLocalObjectStore(root string) (*LocalObjectStore, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("storage root is required")
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, err
	}
	return &LocalObjectStore{root: root}, nil
}

func (s *LocalObjectStore) objectPath(objectKey string) (string, error) {
	clean := filepath.Clean(strings.ReplaceAll(objectKey, "\\", "/"))
	if clean == "." || strings.HasPrefix(clean, "..") || strings.Contains(clean, ":") {
		return "", fmt.Errorf("invalid object key")
	}
	full := filepath.Join(s.root, filepath.FromSlash(clean))
	if !strings.HasPrefix(full, s.root) {
		return "", fmt.Errorf("object key escapes storage root")
	}
	return full, nil
}

func (s *LocalObjectStore) Put(ctx context.Context, objectKey string, reader io.Reader, maxBytes int64) (int64, error) {
	_ = ctx
	path, err := s.objectPath(objectKey)
	if err != nil {
		return 0, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "upload-*")
	if err != nil {
		return 0, err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	limited := io.LimitReader(reader, maxBytes+1)
	written, err := io.Copy(tmp, limited)
	if err != nil {
		_ = tmp.Close()
		return 0, err
	}
	if err := tmp.Close(); err != nil {
		return 0, err
	}
	if written > maxBytes {
		return 0, fmt.Errorf("file exceeds max size")
	}
	if err := os.Link(tmpName, path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return 0, ErrObjectExists
		}
		return 0, err
	}
	return written, nil
}

func (s *LocalObjectStore) Get(ctx context.Context, objectKey string) (io.ReadCloser, error) {
	_ = ctx
	path, err := s.objectPath(objectKey)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrObjectNotFound
		}
		return nil, ErrDownloadFailed
	}
	return file, nil
}

func (s *LocalObjectStore) Metadata(ctx context.Context, objectKey string) (ObjectMetadata, error) {
	_ = ctx
	path, err := s.objectPath(objectKey)
	if err != nil {
		return ObjectMetadata{}, err
	}
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return ObjectMetadata{}, ErrObjectNotFound
		}
		return ObjectMetadata{}, ErrStorageUnavailable
	}
	return ObjectMetadata{SizeBytes: info.Size()}, nil
}

func (s *LocalObjectStore) Delete(ctx context.Context, objectKey string) error {
	_ = ctx
	path, err := s.objectPath(objectKey)
	if err != nil {
		return err
	}
	err = os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func (s *LocalObjectStore) Exists(ctx context.Context, objectKey string) (bool, error) {
	_ = ctx
	path, err := s.objectPath(objectKey)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
