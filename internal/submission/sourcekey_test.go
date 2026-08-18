package submission

import "testing"

func TestSourceKey_UsesIdentityWhenBothFieldsPresent(t *testing.T) {
	k1 := SourceKey([]byte(`{"a":1}`), FamilyKubernetes, "audit-1", "ResponseComplete", "")
	k2 := SourceKey([]byte(`{"a":2}`), FamilyKubernetes, "audit-1", "ResponseComplete", "")
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
		k := SourceKey(raw, FamilyKubernetes, c.auditID, c.stage, "")
		if len(k) < 4 || k[:4] != "raw:" {
			t.Errorf("auditID=%q stage=%q: expected raw: prefix, got %q", c.auditID, c.stage, k)
		}
	}
}

func TestSourceKey_RawHashDiffersForDifferentBytes(t *testing.T) {
	k1 := SourceKey([]byte(`{"a":1}`), FamilyKubernetes, "", "", "")
	k2 := SourceKey([]byte(`{"a":2}`), FamilyKubernetes, "", "", "")
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
	k1 := SourceKey(nil, FamilyKubernetes, "a", "b/c", "")
	k2 := SourceKey(nil, FamilyKubernetes, "a/b", "c", "")
	if k1 == k2 {
		t.Errorf("expected distinct keys for (%q,%q) and (%q,%q), got the same key %q", "a", "b/c", "a/b", "c", k1)
	}
}

func TestSourceKey_IdentityAndRawHashPrefixesNeverCollide(t *testing.T) {
	idKey := SourceKey([]byte(`{}`), FamilyKubernetes, "audit-1", "ResponseComplete", "")
	rawKey := SourceKey([]byte(`{}`), FamilyKubernetes, "", "", "")
	if idKey == rawKey {
		t.Errorf("expected id: and raw: derivation schemes to never produce the same key, got %q for both", idKey)
	}
}

// TestSourceKey_CloudTrailUsesEventIDPrefix proves the CloudTrail-family
// branch keys solely on sourceEventID (ADR-0006 Consequences), ignoring
// rawEvent -- the same "identity, not content, determines the key" shape
// as the Kubernetes id: branch.
func TestSourceKey_CloudTrailUsesEventIDPrefix(t *testing.T) {
	k1 := SourceKey([]byte(`{"a":1}`), FamilyCloudTrail, "", "", "evt-1")
	k2 := SourceKey([]byte(`{"a":2}`), FamilyCloudTrail, "", "", "evt-1")
	if k1 != k2 {
		t.Errorf("expected CloudTrail identity-based key to ignore rawEvent, got %q and %q", k1, k2)
	}
	if len(k1) < 4 || k1[:3] != "ct:" {
		t.Errorf("expected CloudTrail identity-based key to have the ct: prefix, got %q", k1)
	}
}

func TestSourceKey_CloudTrailFallsBackToRawHashWhenEventIDMissing(t *testing.T) {
	k := SourceKey([]byte(`{"a":1}`), FamilyCloudTrail, "", "", "")
	if len(k) < 4 || k[:4] != "raw:" {
		t.Errorf("expected raw: prefix when eventID is missing, got %q", k)
	}
}

// TestSourceKey_ThreePrefixesNeverCollide extends the two-scheme collision
// check above to all three derivation schemes now that CloudTrail's ct:
// branch exists: same raw bytes, three different (family, identity)
// combinations, three disjoint prefixes.
func TestSourceKey_ThreePrefixesNeverCollide(t *testing.T) {
	raw := []byte(`{}`)
	idKey := SourceKey(raw, FamilyKubernetes, "audit-1", "ResponseComplete", "")
	ctKey := SourceKey(raw, FamilyCloudTrail, "", "", "evt-1")
	rawKey := SourceKey(raw, FamilyKubernetes, "", "", "")

	if idKey == ctKey || idKey == rawKey || ctKey == rawKey {
		t.Errorf("expected id:/ct:/raw: keys to be pairwise distinct, got id=%q ct=%q raw=%q", idKey, ctKey, rawKey)
	}
}

// TestSourceKey_SameIdentityDifferentFamilyNeverCollides proves a
// Kubernetes submission and a CloudTrail submission can never derive the
// same key even if their identity strings happened to be textually equal
// -- family is part of which branch runs, not merely an input alongside
// the identity fields.
func TestSourceKey_SameIdentityDifferentFamilyNeverCollides(t *testing.T) {
	raw := []byte(`{}`)
	k8sKey := SourceKey(raw, FamilyKubernetes, "shared-id", "ResponseComplete", "")
	ctKey := SourceKey(raw, FamilyCloudTrail, "", "", "shared-id")
	if k8sKey == ctKey {
		t.Errorf("expected Kubernetes id: key and CloudTrail ct: key to differ even for the same identity string, got %q for both", k8sKey)
	}
}

// --- raw: fallback namespace-safety regression (confirmed-MEDIUM audit
// finding fix): the fallback branch now mixes family into its hashed
// input, so two submissions from different families that both lack any
// extractable identity can no longer derive the same source_key merely
// for sharing byte-identical raw content.

// TestSourceKey_RawFallback_DifferentFamiliesSameBytes_ProduceDistinctKeys
// is the direct regression proof for the fix: before it, a Kubernetes
// submission with no auditID/auditStage and a CloudTrail submission with
// no sourceEventID, sharing identical raw bytes, derived the identical
// "raw:"-prefixed key.
func TestSourceKey_RawFallback_DifferentFamiliesSameBytes_ProduceDistinctKeys(t *testing.T) {
	raw := []byte(`{}`)
	k8sKey := SourceKey(raw, FamilyKubernetes, "", "", "")
	ctKey := SourceKey(raw, FamilyCloudTrail, "", "", "")

	if k8sKey[:4] != "raw:" || ctKey[:4] != "raw:" {
		t.Fatalf("expected both keys to use the raw: fallback, got k8s=%q cloudtrail=%q", k8sKey, ctKey)
	}
	if k8sKey == ctKey {
		t.Errorf("expected the raw: fallback to be namespace-safe across families, got the same key %q for both a Kubernetes and a CloudTrail submission with identical raw bytes and no extractable identity", k8sKey)
	}
}

// TestSourceKey_RawFallback_SameFamilySameBytes_StillIdempotent proves
// the fix preserves same-family idempotent redelivery: two Kubernetes
// raw-fallback calls with identical content must still produce the same
// key (requirement 1).
func TestSourceKey_RawFallback_SameFamilySameBytes_StillIdempotent(t *testing.T) {
	raw := []byte(`{"malformed":true}`)
	k1 := SourceKey(raw, FamilyKubernetes, "", "", "")
	k2 := SourceKey(raw, FamilyKubernetes, "", "", "")
	if k1 != k2 {
		t.Errorf("expected two Kubernetes raw-fallback calls with identical content to produce the same key, got %q and %q", k1, k2)
	}
	if k1[:4] != "raw:" {
		t.Errorf("expected the raw: fallback prefix, got %q", k1)
	}
}

// TestSourceKey_CloudTrailRawFallback_SameBytes_StillIdempotent is
// TestSourceKey_RawFallback_SameFamilySameBytes_StillIdempotent's
// CloudTrail-family counterpart.
func TestSourceKey_CloudTrailRawFallback_SameBytes_StillIdempotent(t *testing.T) {
	raw := []byte(`{"malformed":true}`)
	k1 := SourceKey(raw, FamilyCloudTrail, "", "", "")
	k2 := SourceKey(raw, FamilyCloudTrail, "", "", "")
	if k1 != k2 {
		t.Errorf("expected two CloudTrail raw-fallback calls with identical content to produce the same key, got %q and %q", k1, k2)
	}
	if k1[:4] != "raw:" {
		t.Errorf("expected the raw: fallback prefix, got %q", k1)
	}
}
