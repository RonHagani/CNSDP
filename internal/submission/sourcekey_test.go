package submission

import "testing"

func TestSourceKey_UsesIdentityWhenBothFieldsPresent(t *testing.T) {
	k1 := SourceKey([]byte(`{"a":1}`), "audit-1", "ResponseComplete")
	k2 := SourceKey([]byte(`{"a":2}`), "audit-1", "ResponseComplete")
	if k1 != k2 {
		t.Errorf("expected identity-based key to ignore rawEvent, got %q and %q", k1, k2)
	}
	if len(k1) < 4 || k1[:3] != "id:" {
		t.Errorf("expected identity-based key to have the id: prefix, got %q", k1)
	}
}

func TestSourceKey_FallsBackToRawHashWhenIdentityMissing(t *testing.T) {
	raw := []byte(`{"a":1}`)
	cases := []struct {
		auditID, stage string
	}{
		{"", ""},
		{"audit-1", ""},
		{"", "ResponseComplete"},
	}
	for _, c := range cases {
		k := SourceKey(raw, c.auditID, c.stage)
		if len(k) < 4 || k[:4] != "raw:" {
			t.Errorf("auditID=%q stage=%q: expected raw: prefix, got %q", c.auditID, c.stage, k)
		}
	}
}

func TestSourceKey_RawHashDiffersForDifferentBytes(t *testing.T) {
	k1 := SourceKey([]byte(`{"a":1}`), "", "")
	k2 := SourceKey([]byte(`{"a":2}`), "", "")
	if k1 == k2 {
		t.Errorf("expected different raw bytes to produce different keys, both were %q", k1)
	}
}

// TestSourceKey_NoCollisionForAmbiguousDelimiterShapedInputs proves the
// canonical-JSON-encoding derivation doesn't collide the way a naive
// "auditID + \"/\" + auditStage" concatenation would: ("a", "b/c") and
// ("a/b", "c") both admit unconditionally (intake never rejects on
// content), so neither string can be assumed free of whatever separator a
// concatenation-based scheme might pick.
func TestSourceKey_NoCollisionForAmbiguousDelimiterShapedInputs(t *testing.T) {
	k1 := SourceKey(nil, "a", "b/c")
	k2 := SourceKey(nil, "a/b", "c")
	if k1 == k2 {
		t.Errorf("expected distinct keys for (%q,%q) and (%q,%q), got the same key %q", "a", "b/c", "a/b", "c", k1)
	}
}

func TestSourceKey_IdentityAndRawHashPrefixesNeverCollide(t *testing.T) {
	idKey := SourceKey([]byte(`{}`), "audit-1", "ResponseComplete")
	rawKey := SourceKey([]byte(`{}`), "", "")
	if idKey == rawKey {
		t.Errorf("expected id: and raw: derivation schemes to never produce the same key, got %q for both", idKey)
	}
}
