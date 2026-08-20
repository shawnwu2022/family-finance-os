package agentadapter

import (
	"testing"
	"time"
)

func TestAuditedServiceDefinitionsAreDefensiveCopies(t *testing.T) {
	service, err := New(&fakeBackend{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	audited, err := NewAudited(service, &fakeAgentAuditRecorder{startID: 1}, time.Now)
	if err != nil {
		t.Fatalf("NewAudited: %v", err)
	}

	first := audited.Definitions()
	if len(first) == 0 {
		t.Fatal("Definitions returned no tools")
	}
	first[0].Description = "mutated"
	if len(first[0].InputSchema) > 0 {
		first[0].InputSchema[0] = 'x'
	}

	second := audited.Definitions()
	if second[0].Description == "mutated" {
		t.Fatal("Definitions leaked mutable description state")
	}
	if len(second[0].InputSchema) == 0 || second[0].InputSchema[0] != '{' {
		t.Fatalf("Definitions leaked mutable schema state: %q", second[0].InputSchema)
	}
}
