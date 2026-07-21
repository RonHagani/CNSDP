package detection

import (
	"bytes"
	"encoding/json"
	"slices"
	"testing"
)

func validBaseline() Definition {
	return Definition{
		Scenario:    "scenario-1",
		Name:        "Test",
		Description: "Test description.",
		Conditions: Conditions{
			Operation: Operation{Resource: "pods"},
			RequiresAny: []Characteristic{
				{ID: "a", Description: "A"},
			},
		},
	}
}

// --- Parse: strict YAML parsing ---

func TestParse_RejectsMalformedYAML(t *testing.T) {
	_, err := Parse([]byte("scenario: [unclosed\n  name: x"))
	if err == nil {
		t.Fatal("expected an error for malformed YAML syntax")
	}
}

func TestParse_RejectsUnknownField(t *testing.T) {
	raw := []byte(`
scenario: scenario-1
name: A
description: B
unexpected_field: oops
conditions:
  operation:
    resource: pods
  requires_any:
    - id: a
      description: b
`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected an error for an unknown top-level field")
	}
}

func TestParse_RejectsDuplicateYAMLKey(t *testing.T) {
	raw := []byte(`
scenario: scenario-1
scenario: scenario-2
name: A
description: B
conditions:
  operation:
    resource: pods
  requires_any:
    - id: a
      description: b
`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected an error for a duplicate YAML mapping key")
	}
}

func TestParse_ValidDocumentSucceeds(t *testing.T) {
	raw := []byte(`
scenario: scenario-1
name: A
description: B
conditions:
  operation:
    resource: pods
    subresource: exec
  requires_any:
    - id: a
      description: b
`)
	def, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if def.Scenario != "scenario-1" || def.Conditions.Operation.Subresource != "exec" {
		t.Errorf("parsed definition missing expected content: %+v", def)
	}
}

// --- Parse: exactly one YAML document ---

func TestParse_RejectsSecondYAMLDocument(t *testing.T) {
	raw := []byte(`
scenario: scenario-1
name: A
description: B
conditions:
  operation:
    resource: pods
  requires_any:
    - id: a
      description: b
---
scenario: scenario-2
name: C
description: D
conditions:
  operation:
    resource: pods
  requires_any:
    - id: c
      description: d
`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected an error for a second YAML document")
	}
}

func TestParse_RejectsTrailingContentAfterDocumentEnd(t *testing.T) {
	raw := []byte(`
scenario: scenario-1
name: A
description: B
conditions:
  operation:
    resource: pods
  requires_any:
    - id: a
      description: b
...
trailing scalar content
`)
	if _, err := Parse(raw); err == nil {
		t.Fatal("expected an error for trailing content after the document-end marker")
	}
}

func TestParse_AcceptsTrailingBlankLinesOnly(t *testing.T) {
	raw := []byte(`
scenario: scenario-1
name: A
description: B
conditions:
  operation:
    resource: pods
  requires_any:
    - id: a
      description: b


`)
	if _, err := Parse(raw); err != nil {
		t.Errorf("expected trailing blank lines alone to be accepted, got: %v", err)
	}
}

// --- Validate ---

func TestValidate_RejectsMissingOrInvalidFields(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Definition)
	}{
		{"empty scenario", func(d *Definition) { d.Scenario = "" }},
		{"unrecognized scenario", func(d *Definition) { d.Scenario = "scenario-4" }},
		{"empty name", func(d *Definition) { d.Name = "" }},
		{"empty description", func(d *Definition) { d.Description = "" }},
		{"empty operation resource", func(d *Definition) { d.Conditions.Operation.Resource = "" }},
		{"empty characteristic id", func(d *Definition) { d.Conditions.RequiresAny[0].ID = "" }},
		{"empty characteristic description", func(d *Definition) { d.Conditions.RequiresAny[0].Description = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			def := validBaseline()
			tc.mutate(&def)
			if err := def.Validate(); err == nil {
				t.Errorf("expected validation to reject: %s", tc.name)
			}
		})
	}
}

func TestValidate_RejectsDuplicateCharacteristicIDWithinList(t *testing.T) {
	def := validBaseline()
	def.Conditions.RequiresAny = []Characteristic{
		{ID: "dup", Description: "first"},
		{ID: "dup", Description: "second"},
	}
	if err := def.Validate(); err == nil {
		t.Fatal("expected validation to reject a duplicate characteristic id within one list")
	}
}

func TestValidate_RejectsDuplicateCharacteristicIDAcrossLists(t *testing.T) {
	def := validBaseline()
	def.Conditions.RequiresAny = []Characteristic{{ID: "dup", Description: "in any"}}
	def.Conditions.RequiresAll = []Characteristic{{ID: "dup", Description: "in all"}}
	if err := def.Validate(); err == nil {
		t.Fatal("expected validation to reject a characteristic id duplicated across requires_any and requires_all")
	}
}

func TestValidate_RejectsBothCharacteristicListsEmpty(t *testing.T) {
	def := validBaseline()
	def.Conditions.RequiresAny = nil
	def.Conditions.RequiresAll = nil
	if err := def.Validate(); err == nil {
		t.Fatal("expected validation to reject a definition with no characteristics")
	}
}

func TestValidate_AcceptsBaseline(t *testing.T) {
	if err := validBaseline().Validate(); err != nil {
		t.Errorf("expected the baseline definition to be valid, got: %v", err)
	}
}

// --- Canonical / RevisionID determinism ---

func TestCanonical_IgnoresFormattingAndKeyOrder(t *testing.T) {
	a := []byte(`
scenario: scenario-1
name: Test
description: Desc.
conditions:
  operation:
    resource: pods
  requires_any:
    - id: a
      description: A
    - id: b
      description: B
`)
	b := []byte(`
# a comment that must not affect the revision
description: Desc.
name: Test
scenario: scenario-1

conditions:
  requires_any:
    - description: A
      id: a
    - description: B
      id: b
  operation:
    resource: pods
`)

	defA, err := Parse(a)
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}
	defB, err := Parse(b)
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}

	canonA, err := defA.Canonical()
	if err != nil {
		t.Fatalf("canonical a: %v", err)
	}
	canonB, err := defB.Canonical()
	if err != nil {
		t.Fatalf("canonical b: %v", err)
	}

	if !bytes.Equal(canonA, canonB) {
		t.Errorf("canonical bytes differ despite identical semantic content:\na=%s\nb=%s", canonA, canonB)
	}
}

func TestCanonical_IgnoresCharacteristicListOrder(t *testing.T) {
	a := []byte(`
scenario: scenario-1
name: Test
description: Desc.
conditions:
  operation:
    resource: pods
  requires_any:
    - id: a
      description: A
    - id: b
      description: B
`)
	b := []byte(`
scenario: scenario-1
name: Test
description: Desc.
conditions:
  operation:
    resource: pods
  requires_any:
    - id: b
      description: B
    - id: a
      description: A
`)

	defA, err := Parse(a)
	if err != nil {
		t.Fatalf("parse a: %v", err)
	}
	defB, err := Parse(b)
	if err != nil {
		t.Fatalf("parse b: %v", err)
	}
	canonA, err := defA.Canonical()
	if err != nil {
		t.Fatalf("canonical a: %v", err)
	}
	canonB, err := defB.Canonical()
	if err != nil {
		t.Fatalf("canonical b: %v", err)
	}

	if !bytes.Equal(canonA, canonB) {
		t.Errorf("canonical bytes differ despite only characteristic order differing:\na=%s\nb=%s", canonA, canonB)
	}
}

func TestCanonical_SemanticChangeProducesDifferentRevision(t *testing.T) {
	base := validBaseline()
	baseCanonical, err := base.Canonical()
	if err != nil {
		t.Fatalf("canonical base: %v", err)
	}
	baseRevision := RevisionID(baseCanonical)

	cases := []struct {
		name   string
		mutate func(*Definition)
	}{
		{"name changed", func(d *Definition) { d.Name = "Different Name" }},
		{"description changed", func(d *Definition) { d.Description = "Different description." }},
		{"operation resource changed", func(d *Definition) { d.Conditions.Operation.Resource = "clusterrolebindings" }},
		{"requires_outcome added", func(d *Definition) { d.Conditions.RequiresOutcome = "success" }},
		{"characteristic description changed", func(d *Definition) { d.Conditions.RequiresAny[0].Description = "changed" }},
		{"characteristic added", func(d *Definition) {
			d.Conditions.RequiresAny = append(d.Conditions.RequiresAny, Characteristic{ID: "extra", Description: "extra"})
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			def := validBaseline()
			tc.mutate(&def)
			canonical, err := def.Canonical()
			if err != nil {
				t.Fatalf("canonical: %v", err)
			}
			revision := RevisionID(canonical)
			if revision == baseRevision {
				t.Errorf("expected a different revision after %s, got the same revision as the baseline", tc.name)
			}
		})
	}
}

// TestCanonical_DoesNotMutateOriginalDefinition is the required proof for
// the c := d shallow-copy correction: Canonical() must sort a clone of
// each characteristic slice, never the original's backing array.
func TestCanonical_DoesNotMutateOriginalDefinition(t *testing.T) {
	def := Definition{
		Scenario:    "scenario-1",
		Name:        "Test",
		Description: "Test description.",
		Conditions: Conditions{
			Operation: Operation{Resource: "pods"},
			RequiresAny: []Characteristic{
				{ID: "zzz_last", Description: "Z"},
				{ID: "aaa_first", Description: "A"},
			},
			RequiresAll: []Characteristic{
				{ID: "mmm_middle", Description: "M"},
				{ID: "bbb_before", Description: "B"},
			},
		},
	}

	originalAnyOrder := idsOf(def.Conditions.RequiresAny)
	originalAllOrder := idsOf(def.Conditions.RequiresAll)

	canonical, err := def.Canonical()
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}

	var decoded Definition
	if err := json.Unmarshal(canonical, &decoded); err != nil {
		t.Fatalf("unmarshal canonical output: %v", err)
	}

	if got, want := idsOf(decoded.Conditions.RequiresAny), []string{"aaa_first", "zzz_last"}; !slices.Equal(got, want) {
		t.Errorf("canonical output requires_any order = %v, want sorted %v", got, want)
	}
	if got, want := idsOf(decoded.Conditions.RequiresAll), []string{"bbb_before", "mmm_middle"}; !slices.Equal(got, want) {
		t.Errorf("canonical output requires_all order = %v, want sorted %v", got, want)
	}

	if got := idsOf(def.Conditions.RequiresAny); !slices.Equal(got, originalAnyOrder) {
		t.Errorf("original Definition's requires_any order changed: got %v, want unchanged %v", got, originalAnyOrder)
	}
	if got := idsOf(def.Conditions.RequiresAll); !slices.Equal(got, originalAllOrder) {
		t.Errorf("original Definition's requires_all order changed: got %v, want unchanged %v", got, originalAllOrder)
	}
}

func idsOf(cs []Characteristic) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.ID
	}
	return out
}
