package service

import "testing"

func TestBillingSyncHTTPErrorMessage(t *testing.T) {
	t.Parallel()
	err := &BillingSyncHTTPError{StatusCode: 409, Body: "conflict"}
	if err.Error() == "" {
		t.Fatal("expected readable error text")
	}
	if got := (&BillingSyncHTTPError{StatusCode: 503}).Error(); got != "billing sync-paid failed: status=503" {
		t.Fatalf("unexpected message: %s", got)
	}
}
