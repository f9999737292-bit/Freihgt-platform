package domain

import "testing"

func TestEnsureTemplateVersionEventCloneSource(t *testing.T) {
	if err := EnsureTemplateVersionEventCloneSource(RfxVersionStatusPublished); err != nil {
		t.Fatalf("published: %v", err)
	}
	if err := EnsureTemplateVersionEventCloneSource(RfxVersionStatusSuperseded); err != nil {
		t.Fatalf("superseded: %v", err)
	}
	if err := EnsureTemplateVersionEventCloneSource(RfxVersionStatusDraft); err == nil {
		t.Fatal("expected draft rejection")
	}
}
