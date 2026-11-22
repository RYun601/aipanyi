package mobilecore

import "testing"

func TestAssignSuggestionBasisSkipsEmptyLedger(t *testing.T) {
	zero := int64(0)
	suggestions := []alertSuggestionDTO{{Price: 50}, {Price: 40}}
	assignSuggestionBasis(suggestions, &zero)
	for i, suggestion := range suggestions {
		if suggestion.PositionRevision != nil {
			t.Fatalf("suggestion %d bound empty ledger revision: %+v", i, suggestion)
		}
	}
	assignSuggestionBasis(suggestions, nil)
	for i, suggestion := range suggestions {
		if suggestion.PositionRevision != nil {
			t.Fatalf("suggestion %d bound nil revision: %+v", i, suggestion)
		}
	}

	revision := int64(7)
	assignSuggestionBasis(suggestions, &revision)
	for i, suggestion := range suggestions {
		if suggestion.PositionRevision == nil || *suggestion.PositionRevision != revision {
			t.Fatalf("suggestion %d did not bind revision %d: %+v", i, revision, suggestion)
		}
	}
}
