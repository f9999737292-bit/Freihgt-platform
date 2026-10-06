package storage

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestS3PutMapsOnlyPreconditionAndConflictAsExists(t *testing.T) {
	key := "tenants/a/documents/b/attachments/c"
	body := []byte("payload")
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusConflict, ErrObjectExists},
		{http.StatusForbidden, ErrUploadFailed},
		{http.StatusInternalServerError, ErrUploadFailed},
	}
	for _, tc := range cases {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("If-None-Match") != "*" {
				t.Errorf("missing If-None-Match")
			}
			w.WriteHeader(tc.status)
		}))
		store := testS3(t, server.URL)
		_, err := store.Put(context.Background(), key, bytes.NewReader(body), int64(len(body)))
		server.Close()
		if !errors.Is(err, tc.want) {
			t.Fatalf("status %d err=%v want %v", tc.status, err, tc.want)
		}
		if tc.want == ErrUploadFailed && errors.Is(err, ErrObjectExists) {
			t.Fatalf("status %d was treated as object collision", tc.status)
		}
	}
}
