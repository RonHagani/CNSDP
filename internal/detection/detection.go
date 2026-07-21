// Package detection reads, strictly parses, validates, and loads the
// three version-controlled detection-definition files (ADR-0004) into
// immutable, revision-identified detection_definitions rows. It contains
// no evaluation logic — matching a normalized event against these
// definitions belongs to a later checkpoint.
package detection

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"

	yaml "go.yaml.in/yaml/v3"

	"cnsdp/definitions"
)

// Definition is one parsed, validated detection-definition file (FR-020,
// FR-021). Both yaml (strict parsing) and json (canonical storage) tags
// are carried on every field.
type Definition struct {
	Scenario    string     `yaml:"scenario" json:"scenario"`
	Name        string     `yaml:"name" json:"name"`
	Description string     `yaml:"description" json:"description"`
	Conditions  Conditions `yaml:"conditions" json:"conditions"`
}

type Conditions struct {
	Operation       Operation        `yaml:"operation" json:"operation"`
	RequiresOutcome string           `yaml:"requires_outcome,omitempty" json:"requires_outcome,omitempty"`
	RequiresAny     []Characteristic `yaml:"requires_any,omitempty" json:"requires_any,omitempty"`
	RequiresAll     []Characteristic `yaml:"requires_all,omitempty" json:"requires_all,omitempty"`
}

type Operation struct {
	Resource    string `yaml:"resource" json:"resource"`
	Subresource string `yaml:"subresource,omitempty" json:"subresource,omitempty"`
	Verb        string `yaml:"verb,omitempty" json:"verb,omitempty"`
}

type Characteristic struct {
	ID          string `yaml:"id" json:"id"`
	Description string `yaml:"description" json:"description"`
}

// approvedScenarios is the closed set PD-04 approved for v0.1. Nothing
// outside this package should declare its own copy.
var approvedScenarios = map[string]bool{
	"scenario-1": true,
	"scenario-2": true,
	"scenario-3": true,
}

// Parse strictly decodes exactly one YAML document into a Definition:
// unknown fields and duplicate mapping keys are rejected (via
// KnownFields), and a second document, trailing content, or any other
// additional decodable value after the first is rejected by requiring the
// decoder's next read to return io.EOF.
func Parse(raw []byte) (Definition, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)

	var def Definition
	if err := dec.Decode(&def); err != nil {
		return Definition{}, fmt.Errorf("detection: parse: %w", err)
	}

	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		return Definition{}, fmt.Errorf("detection: parse: exactly one YAML document is required")
	}

	return def, nil
}

// Validate checks a parsed Definition against the minimum required
// structure. It does not mutate d.
func (d Definition) Validate() error {
	if d.Scenario == "" {
		return fmt.Errorf("detection: validate: scenario is required")
	}
	if !approvedScenarios[d.Scenario] {
		return fmt.Errorf("detection: validate: %q is not an approved scenario", d.Scenario)
	}
	if d.Name == "" {
		return fmt.Errorf("detection: validate: name is required")
	}
	if d.Description == "" {
		return fmt.Errorf("detection: validate: description is required")
	}
	if d.Conditions.Operation.Resource == "" {
		return fmt.Errorf("detection: validate: conditions.operation.resource is required")
	}
	if len(d.Conditions.RequiresAny) == 0 && len(d.Conditions.RequiresAll) == 0 {
		return fmt.Errorf("detection: validate: at least one of requires_any or requires_all must be non-empty")
	}

	seen := make(map[string]bool)
	checkCharacteristics := func(list []Characteristic, field string) error {
		for _, c := range list {
			if c.ID == "" {
				return fmt.Errorf("detection: validate: a %s characteristic id is required", field)
			}
			if c.Description == "" {
				return fmt.Errorf("detection: validate: %s characteristic %q: description is required", field, c.ID)
			}
			if seen[c.ID] {
				return fmt.Errorf("detection: validate: duplicate characteristic id %q", c.ID)
			}
			seen[c.ID] = true
		}
		return nil
	}
	if err := checkCharacteristics(d.Conditions.RequiresAny, "requires_any"); err != nil {
		return err
	}
	if err := checkCharacteristics(d.Conditions.RequiresAll, "requires_all"); err != nil {
		return err
	}

	return nil
}

// Canonical produces the deterministic byte representation of a validated
// Definition, used both as the sole input to RevisionID and as the stored
// content. requires_any and requires_all are sorted by characteristic ID
// so that reordering them in the source file never changes the result;
// every other field's order is fixed by Go struct field declaration
// order, which encoding/json already respects deterministically.
//
// Canonical has no observable side effects on d: the characteristic
// slices are cloned before sorting, never sorted in place.
func (d Definition) Canonical() ([]byte, error) {
	c := d
	c.Conditions.RequiresAny = sortedByID(d.Conditions.RequiresAny)
	c.Conditions.RequiresAll = sortedByID(d.Conditions.RequiresAll)
	return json.Marshal(c)
}

func sortedByID(in []Characteristic) []Characteristic {
	out := slices.Clone(in)
	slices.SortFunc(out, func(a, b Characteristic) int {
		return strings.Compare(a.ID, b.ID)
	})
	return out
}

// RevisionID derives the deterministic revision identifier from a
// Definition's canonical bytes. PostgreSQL's JSONB storage does not
// guarantee byte-for-byte preservation of what was inserted, so this
// value is never recomputed by hashing content retrieved from the
// database directly -- a later verification must re-parse the retrieved
// content into a Definition and call Canonical() again.
func RevisionID(canonical []byte) string {
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:])
}

// insertDefinition performs one conflict-safe, idempotent insert. Reused
// by LoadFS's normal loop and called directly by tests that need to
// exercise the same real insert path a stage transaction uses.
func insertDefinition(ctx context.Context, tx *sql.Tx, scenario, revision string, content []byte) error {
	_, err := tx.ExecContext(ctx,
		`INSERT INTO detection_definitions (scenario, revision, content) VALUES ($1, $2, $3)
		 ON CONFLICT (scenario, revision) DO NOTHING`,
		scenario, revision, content)
	if err != nil {
		return fmt.Errorf("detection: insert %s/%s: %w", scenario, revision, err)
	}
	return nil
}

type loadedDefinition struct {
	scenario  string
	revision  string
	canonical []byte
}

// LoadFS reads every *.yaml file in fsys, parses and validates each one,
// and confirms the resulting scenario set is exactly the three approved
// scenarios -- all entirely in memory, before any database interaction.
// Only once every file passes does it open one transaction and insert the
// complete set. If anything fails at any point, nothing is written.
func LoadFS(ctx context.Context, db *sql.DB, fsys fs.FS) error {
	names, err := fs.Glob(fsys, "*.yaml")
	if err != nil {
		return fmt.Errorf("detection: list definition files: %w", err)
	}
	slices.Sort(names) // deterministic processing order

	var results []loadedDefinition
	seenScenarios := make(map[string]bool)

	for _, name := range names {
		raw, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("detection: read %s: %w", name, err)
		}

		def, err := Parse(raw)
		if err != nil {
			return fmt.Errorf("detection: %s: %w", name, err)
		}
		if err := def.Validate(); err != nil {
			return fmt.Errorf("detection: %s: %w", name, err)
		}
		if seenScenarios[def.Scenario] {
			return fmt.Errorf("detection: %s: duplicate scenario %q across definition files", name, def.Scenario)
		}
		seenScenarios[def.Scenario] = true

		canonical, err := def.Canonical()
		if err != nil {
			return fmt.Errorf("detection: %s: canonicalize: %w", name, err)
		}
		results = append(results, loadedDefinition{
			scenario:  def.Scenario,
			revision:  RevisionID(canonical),
			canonical: canonical,
		})
	}

	for scenario := range approvedScenarios {
		if !seenScenarios[scenario] {
			return fmt.Errorf("detection: missing required scenario %q", scenario)
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("detection: begin load tx: %w", err)
	}
	defer tx.Rollback()

	for _, r := range results {
		if err := insertDefinition(ctx, tx, r.scenario, r.revision, r.canonical); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("detection: commit load tx: %w", err)
	}
	return nil
}

// Load is the production entry point: loads the embedded, version-controlled
// definitions (ADR-0004), called once at application startup.
func Load(ctx context.Context, db *sql.DB) error {
	return LoadFS(ctx, db, definitions.FS)
}
