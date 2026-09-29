//go:build integration

package edo03schema

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	i1UpFile   = "000087_edo_0_3_i1_schema_foundation.up.sql"
	i1DownFile = "000087_edo_0_3_i1_schema_foundation.down.sql"
	digestHex  = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

var adminURL string

func TestMain(m *testing.M) {
	if existing := strings.TrimSpace(os.Getenv("TEST_DATABASE_URL")); existing != "" {
		adminURL = existing
		os.Exit(m.Run())
	}
	port := freePort()
	pg := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Port(uint32(port)).
		RuntimePath(filepath.Join(os.TempDir(), "edo-i1-pg-runtime")).
		DataPath(filepath.Join(os.TempDir(), "edo-i1-pg-data-"+uuid.NewString()[:8])).
		Database("freight_doc_i1").
		Username("freight").Password("freight").
		Version(embeddedpostgres.V16))
	if err := pg.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "embedded postgres: %v\n", err)
		os.Exit(1)
	}
	adminURL = fmt.Sprintf("postgres://freight:freight@127.0.0.1:%d/postgres?sslmode=disable", port)
	code := m.Run()
	_ = pg.Stop()
	os.Exit(code)
}

func freePort() int {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		panic(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func newPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	dbName := "edo_i1_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	testURL, cleanup, err := createDB(ctx, adminURL, dbName)
	if err != nil {
		t.Fatalf("create db: %v", err)
	}
	t.Cleanup(func() { cleanup(context.Background()) })
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	if err := applyBase(ctx, pool); err != nil {
		t.Fatalf("base migrations: %v", err)
	}
	return pool
}

func createDB(ctx context.Context, admin, dbName string) (string, func(context.Context), error) {
	cfg, err := pgxpool.ParseConfig(admin)
	if err != nil {
		return "", nil, err
	}
	adminPool, err := pgxpool.NewWithConfig(ctx, cfg.Copy())
	if err != nil {
		return "", nil, err
	}
	defer adminPool.Close()
	if _, err := adminPool.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{dbName}.Sanitize()); err != nil {
		return "", nil, err
	}
	testCfg := cfg.Copy()
	testCfg.ConnConfig.Database = dbName
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		testCfg.ConnConfig.User, testCfg.ConnConfig.Password,
		testCfg.ConnConfig.Host, testCfg.ConnConfig.Port, dbName)
	cleanup := func(cctx context.Context) {
		cp, err := pgxpool.NewWithConfig(cctx, cfg.Copy())
		if err != nil {
			return
		}
		defer cp.Close()
		_, _ = cp.Exec(cctx, "DROP DATABASE IF EXISTS "+pgx.Identifier{dbName}.Sanitize()+" WITH (FORCE)")
	}
	return url, cleanup, nil
}

func applyBase(ctx context.Context, pool *pgxpool.Pool) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.up.sql"))
	if err != nil {
		return err
	}
	sort.Strings(files)
	for _, file := range files {
		num := migrationNumber(filepath.Base(file))
		if num > 14 && num != 33 {
			continue
		}
		if err := execFile(ctx, pool, file); err != nil {
			return err
		}
	}
	return nil
}

func applyI1(ctx context.Context, pool *pgxpool.Pool, down bool) error {
	dir, err := migrationsDir()
	if err != nil {
		return err
	}
	name := i1UpFile
	if down {
		name = i1DownFile
	}
	return execFile(ctx, pool, filepath.Join(dir, name))
}

func execFile(ctx context.Context, pool *pgxpool.Pool, path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if _, err := pool.Exec(ctx, string(content)); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return nil
}

func migrationsDir() (string, error) {
	for _, candidate := range []string{
		filepath.Join("..", "..", "..", "..", "infrastructure", "migrations"),
		filepath.Join("..", "..", "..", "..", "..", "infrastructure", "migrations"),
	} {
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("migrations not found")
}

func migrationNumber(filename string) int {
	parts := strings.SplitN(filename, "_", 2)
	var n int
	fmt.Sscanf(parts[0], "%d", &n)
	return n
}

func mustInsertDocument(t *testing.T, pool *pgxpool.Pool, tenant uuid.UUID, number, status string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO documents.documents (
			id, tenant_id, document_number, document_type, document_status, owner_company_id
		) VALUES ($1,$2,$3,'ACT',$4,$5)`,
		id, tenant, number, status, uuid.New())
	if err != nil {
		t.Fatalf("insert document: %v", err)
	}
	return id
}

func mustInsertVersion(t *testing.T, pool *pgxpool.Pool, documentID uuid.UUID, number int) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO documents.document_versions (id, document_id, version_number)
		VALUES ($1,$2,$3)`, id, documentID, number)
	if err != nil {
		t.Fatalf("insert version: %v", err)
	}
	return id
}

func mustInsertSession(t *testing.T, pool *pgxpool.Pool, tenant, documentID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO documents.signing_sessions (id, tenant_id, document_id)
		VALUES ($1,$2,$3)`, id, tenant, documentID)
	if err != nil {
		t.Fatalf("insert session: %v", err)
	}
	return id
}

func mustInsertSignature(t *testing.T, pool *pgxpool.Pool, tenant, sessionID, documentID uuid.UUID, versionID *uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	algo, digestValue, fingerprint := digestArgs(versionID)
	_, err := pool.Exec(context.Background(), `
		INSERT INTO documents.signatures (
			id, tenant_id, signing_session_id, document_id, signature_type, verification_status,
			document_version_id, content_digest_algorithm, content_digest_value, certificate_fingerprint
		) VALUES ($1,$2,$3,$4,'SIMPLE_ELECTRONIC','PENDING',$5,$6,$7,$8)`,
		id, tenant, sessionID, documentID, versionID, algo, digestValue, fingerprint)
	if err != nil {
		t.Fatalf("insert signature: %v", err)
	}
	return id
}

func digestArgs(versionID *uuid.UUID) (any, any, any) {
	if versionID == nil {
		return nil, nil, nil
	}
	return "SHA-256", digestHex, "fp-test"
}

func mustInsertLegacySignature(t *testing.T, pool *pgxpool.Pool, tenant, sessionID, documentID uuid.UUID) uuid.UUID {
	t.Helper()
	id := uuid.New()
	_, err := pool.Exec(context.Background(), `
		INSERT INTO documents.signatures (
			id, tenant_id, signing_session_id, document_id, signature_type, verification_status
		) VALUES ($1,$2,$3,$4,'SIMPLE_ELECTRONIC','PENDING')`,
		id, tenant, sessionID, documentID)
	if err != nil {
		t.Fatalf("insert legacy signature: %v", err)
	}
	return id
}

func deny(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("expected database rejection")
	}
}

func allow(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("expected success: %v", err)
	}
}
