package domain

import "testing"

func TestSafeFileNameRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../secret", `..\secret`, "/etc/passwd", "a/b.pdf", "", ".."} {
		if _, err := SafeFileName(name); err == nil {
			t.Fatalf("accepted %q", name)
		}
	}
	got, err := SafeFileName("act.pdf")
	if err != nil || got != "act.pdf" {
		t.Fatalf("got %q err %v", got, err)
	}
}

func TestServerSHA256IgnoresCaseAndRejectsUpper(t *testing.T) {
	sum := SHA256Hex([]byte("%PDF-1.7"))
	if !ValidSHA256(sum) {
		t.Fatal("server hash should be lowercase hex")
	}
	if ValidSHA256("ABCD") {
		t.Fatal("short or upper hash accepted")
	}
}
