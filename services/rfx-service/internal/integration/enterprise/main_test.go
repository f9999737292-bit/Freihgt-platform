//go:build integration

package enterprise

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	code := m.Run()
	if os.Getenv("TEST_DATABASE_URL") == "" {
		fmt.Println("REAL_POSTGRES_TESTS=SKIP TEST_DATABASE_URL not set")
		os.Exit(code)
	}
	if code == 0 {
		fmt.Println("REAL_POSTGRES_TESTS=PASS")
		fmt.Println("SKIPPED_REAL_POSTGRES_TESTS=0")
	} else {
		fmt.Printf("REAL_POSTGRES_TESTS=FAIL exit_code=%d\n", code)
	}
	os.Exit(code)
}
