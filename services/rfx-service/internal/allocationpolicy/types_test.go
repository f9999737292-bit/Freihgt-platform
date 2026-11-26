package allocationpolicy

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDecimal_RejectsJSONNumber(t *testing.T) {
	var d Decimal
	err := json.Unmarshal([]byte(`1.25`), &d)
	if err == nil {
		t.Fatal("expected reject of JSON number")
	}
	got := err.Error()
	if !strings.Contains(got, "float64") && !strings.Contains(got, "JSON string") {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestDecimal_AcceptsJSONString(t *testing.T) {
	var d Decimal
	if err := json.Unmarshal([]byte(`"1.25"`), &d); err != nil {
		t.Fatal(err)
	}
	if d.String() != "1.25" {
		t.Fatalf("got %s", d.String())
	}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"1.25"` {
		t.Fatalf("marshal=%s", b)
	}
}
