package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestLocalPutCreateIfAbsent(t *testing.T) {
	store, err := NewLocalObjectStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	key := "tenants/a/documents/b/attachments/c"
	original := []byte("payload-a")
	if _, err := store.Put(context.Background(), key, bytes.NewReader(original), 100); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(context.Background(), key, bytes.NewReader([]byte("payload-b")), 100); !errors.Is(err, ErrObjectExists) {
		t.Fatalf("second put err=%v", err)
	}
	body, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	got, _ := io.ReadAll(body)
	if !bytes.Equal(got, original) {
		t.Fatalf("original bytes changed to %q", got)
	}
}

func TestLocalPutConcurrentCreateIfAbsent(t *testing.T) {
	root := t.TempDir()
	store, err := NewLocalObjectStore(root)
	if err != nil {
		t.Fatal(err)
	}
	key := "tenants/a/documents/b/attachments/c"
	payloads := [][]byte{
		[]byte("payload-a"),
		[]byte("payload-b"),
		[]byte("payload-c"),
		[]byte("payload-d"),
	}
	var wg sync.WaitGroup
	var mu sync.Mutex
	successes, collisions, other := 0, 0, 0
	wg.Add(len(payloads))
	for _, payload := range payloads {
		payload := payload
		go func() {
			defer wg.Done()
			_, err := store.Put(context.Background(), key, bytes.NewReader(payload), 100)
			mu.Lock()
			defer mu.Unlock()
			switch {
			case err == nil:
				successes++
			case errors.Is(err, ErrObjectExists):
				collisions++
			default:
				other++
			}
		}()
	}
	wg.Wait()
	if successes != 1 || collisions != len(payloads)-1 || other != 0 {
		t.Fatalf("successes=%d collisions=%d other=%d", successes, collisions, other)
	}
	body, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	got, _ := io.ReadAll(body)
	matched := false
	for _, payload := range payloads {
		if bytes.Equal(got, payload) {
			matched = true
		}
	}
	if !matched {
		t.Fatalf("final object is not one complete payload: %q", got)
	}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && filepath.Base(path) != "c" && len(filepath.Base(path)) >= 7 && filepath.Base(path)[:7] == "upload-" {
			t.Errorf("temporary upload file left behind: %s", entry.Name())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
