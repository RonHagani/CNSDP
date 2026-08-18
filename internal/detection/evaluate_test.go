package detection

import (
	"io/fs"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"

	"cnsdp/definitions"
	"cnsdp/internal/normalization"
)

// realDefinition loads and parses one of the actual, approved,
// version-controlled definition files -- proving the generic engine
// against real product content, not a synthetic stand-in.
func realDefinition(t *testing.T, filename string) Definition {
	t.Helper()
	raw, err := fs.ReadFile(definitions.FS, filename)
	if err != nil {
		t.Fatalf("read %s: %v", filename, err)
	}
	def, err := Parse(raw)
	if err != nil {
		t.Fatalf("parse %s: %v", filename, err)
	}
	if err := def.Validate(); err != nil {
		t.Fatalf("validate %s: %v", filename, err)
	}
	return def
}

// --- operationMatches ---

func TestOperationMatches(t *testing.T) {
	tests := []struct {
		name  string
		op    Operation
		event normalization.Event
		want  bool
	}{
		{
			"resource wildcard when empty",
			Operation{},
			normalization.Event{Target: normalization.Target{Resource: "anything"}},
			true,
		},
		{
			"resource mismatch",
			Operation{Resource: "pods"},
			normalization.Event{Target: normalization.Target{Resource: "configmaps"}},
			false,
		},
		{
			"verb wildcard when empty",
			Operation{Resource: "pods"},
			normalization.Event{Target: normalization.Target{Resource: "pods"}, Operation: normalization.Operation{Verb: "delete"}},
			true,
		},
		{
			"verb mismatch",
			Operation{Resource: "pods", Verb: "create"},
			normalization.Event{Target: normalization.Target{Resource: "pods"}, Operation: normalization.Operation{Verb: "delete"}},
			false,
		},
		{
			"subresource is never a wildcard: empty definition subresource requires empty event subresource",
			Operation{Resource: "pods", Verb: "create"},
			normalization.Event{Target: normalization.Target{Resource: "pods", Subresource: "exec"}, Operation: normalization.Operation{Verb: "create"}},
			false,
		},
		{
			"subresource exact match required",
			Operation{Resource: "pods", Subresource: "exec"},
			normalization.Event{Target: normalization.Target{Resource: "pods", Subresource: "exec"}},
			true,
		},
		{
			"subresource mismatch",
			Operation{Resource: "pods", Subresource: "exec"},
			normalization.Event{Target: normalization.Target{Resource: "pods", Subresource: "log"}},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := operationMatches(tt.op, tt.event); got != tt.want {
				t.Errorf("operationMatches(%+v, %+v) = %v, want %v", tt.op, tt.event, got, tt.want)
			}
		})
	}
}

// --- outcomeSatisfied ---

// TestOutcomeSatisfied proves outcomeSatisfied reads only Successful --
// the source-neutral signal normalization derives per family -- and never
// branches on any source-specific field (Code, ErrorCode). The 200-299
// derivation rule itself now lives in normalization (see
// TestOutcomeOf_SuccessfulDerivation in internal/normalization), not here:
// this evaluator is deliberately ignorant of how any one family encodes
// success.
func TestOutcomeSatisfied(t *testing.T) {
	tests := []struct {
		name       string
		requires   string
		successful bool
		want       bool
	}{
		{"no constraint, unsuccessful", "", false, true},
		{"no constraint, successful", "", true, true},
		{"success, successful", "success", true, true},
		{"success, unsuccessful", "success", false, false},
		{"unrecognized constraint fails closed", "bogus", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := normalization.Event{Outcome: normalization.Outcome{Successful: tt.successful}}
			if got := outcomeSatisfied(tt.requires, event); got != tt.want {
				t.Errorf("outcomeSatisfied(%q, successful=%v) = %v, want %v", tt.requires, tt.successful, got, tt.want)
			}
		})
	}
}

// --- characteristicSet ---

func TestCharacteristicSet(t *testing.T) {
	t.Run("empty event yields empty set", func(t *testing.T) {
		got := characteristicSet(normalization.Event{})
		if len(got) != 0 {
			t.Errorf("characteristicSet = %+v, want empty", got)
		}
	})

	t.Run("exec characteristics", func(t *testing.T) {
		event := normalization.Event{Exec: &normalization.ExecCharacteristics{Stdin: true, TTY: false}}
		got := characteristicSet(event)
		if got["stdin_streaming"] != true || got["tty_allocation"] != false {
			t.Errorf("characteristicSet = %+v, want stdin_streaming=true tty_allocation=false", got)
		}
	})

	t.Run("pod creation characteristics", func(t *testing.T) {
		event := normalization.Event{PodCreation: &normalization.PodCreationCharacteristics{
			Privileged: true, HostNetwork: false, HostPID: true, HostIPC: false, HostPathVolume: true,
		}}
		got := characteristicSet(event)
		want := map[string]bool{
			"privileged_container": true, "host_network": false, "host_pid": true,
			"host_ipc": false, "host_path_volume": true,
		}
		for k, v := range want {
			if got[k] != v {
				t.Errorf("characteristicSet[%q] = %v, want %v", k, got[k], v)
			}
		}
	})

	t.Run("cluster role binding: exact cluster-admin match required", func(t *testing.T) {
		match := normalization.Event{ClusterRoleBinding: &normalization.ClusterRoleBindingCharacteristics{
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "cluster-admin"},
		}}
		if !characteristicSet(match)["role_ref_cluster_admin"] {
			t.Error("role_ref_cluster_admin = false, want true for an exact cluster-admin ClusterRole reference")
		}

		wrongName := normalization.Event{ClusterRoleBinding: &normalization.ClusterRoleBindingCharacteristics{
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "edit"},
		}}
		if characteristicSet(wrongName)["role_ref_cluster_admin"] {
			t.Error("role_ref_cluster_admin = true, want false for a non-cluster-admin role")
		}

		wrongKind := normalization.Event{ClusterRoleBinding: &normalization.ClusterRoleBindingCharacteristics{
			RoleRef: rbacv1.RoleRef{Kind: "Role", Name: "cluster-admin"},
		}}
		if characteristicSet(wrongKind)["role_ref_cluster_admin"] {
			t.Error("role_ref_cluster_admin = true, want false when Kind is not ClusterRole")
		}
	})
}

// --- evaluate, against the real, approved definition files ---

func TestEvaluate_Scenario1_MatchesRegardlessOfOutcome(t *testing.T) {
	def := realDefinition(t, "scenario-1.yaml")
	event := normalization.Event{
		Target:    normalization.Target{Resource: "pods", Subresource: "exec"},
		Operation: normalization.Operation{Verb: "get"},
		Outcome:   normalization.Outcome{Code: 101}, // not a 2xx -- must still match (FR-024)
		Exec:      &normalization.ExecCharacteristics{Stdin: true, TTY: false},
	}

	matched, satisfied := evaluate(event, def)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if len(satisfied) != 1 || satisfied[0].ID != "stdin_streaming" {
		t.Errorf("satisfied = %+v, want exactly [stdin_streaming]", satisfied)
	}
}

func TestEvaluate_Scenario1_NoCharacteristicsPresent_NoMatch(t *testing.T) {
	def := realDefinition(t, "scenario-1.yaml")
	event := normalization.Event{
		Target:    normalization.Target{Resource: "pods", Subresource: "exec"},
		Operation: normalization.Operation{Verb: "get"},
		Exec:      &normalization.ExecCharacteristics{Stdin: false, TTY: false},
	}

	if matched, _ := evaluate(event, def); matched {
		t.Error("matched = true, want false when neither documented characteristic is present")
	}
}

func TestEvaluate_Scenario2_MatchesOnSuccessWithAnyCharacteristic(t *testing.T) {
	def := realDefinition(t, "scenario-2.yaml")
	event := normalization.Event{
		Target:      normalization.Target{Resource: "pods"},
		Operation:   normalization.Operation{Verb: "create"},
		Outcome:     normalization.Outcome{Code: 201, Successful: true},
		PodCreation: &normalization.PodCreationCharacteristics{Privileged: true, HostIPC: true},
	}

	matched, satisfied := evaluate(event, def)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	gotIDs := map[string]bool{}
	for _, c := range satisfied {
		gotIDs[c.ID] = true
	}
	if !gotIDs["privileged_container"] || !gotIDs["host_ipc"] || len(gotIDs) != 2 {
		t.Errorf("satisfied = %+v, want exactly privileged_container and host_ipc", satisfied)
	}
}

// TestEvaluate_Scenario2_NegativeControl_NoHighRiskCharacteristics is the
// AC-011 negative control: a successful Pod creation with none of the
// five documented characteristics must not match.
func TestEvaluate_Scenario2_NegativeControl_NoHighRiskCharacteristics(t *testing.T) {
	def := realDefinition(t, "scenario-2.yaml")
	event := normalization.Event{
		Target:      normalization.Target{Resource: "pods"},
		Operation:   normalization.Operation{Verb: "create"},
		Outcome:     normalization.Outcome{Code: 201, Successful: true},
		PodCreation: &normalization.PodCreationCharacteristics{},
	}

	if matched, _ := evaluate(event, def); matched {
		t.Error("matched = true, want false for a negative-control Pod with no high-risk characteristics")
	}
}

func TestEvaluate_Scenario2_UnsuccessfulCreation_NoMatchDespiteCharacteristics(t *testing.T) {
	def := realDefinition(t, "scenario-2.yaml")
	event := normalization.Event{
		Target:      normalization.Target{Resource: "pods"},
		Operation:   normalization.Operation{Verb: "create"},
		Outcome:     normalization.Outcome{Code: 403}, // denied -- not a successful creation
		PodCreation: &normalization.PodCreationCharacteristics{Privileged: true},
	}

	if matched, _ := evaluate(event, def); matched {
		t.Error("matched = true, want false: scenario 2 requires a successful creation (FR-025)")
	}
}

func TestEvaluate_Scenario3_MatchesClusterAdminGrant(t *testing.T) {
	def := realDefinition(t, "scenario-3.yaml")
	event := normalization.Event{
		Target:    normalization.Target{Resource: "clusterrolebindings"},
		Operation: normalization.Operation{Verb: "create"},
		Outcome:   normalization.Outcome{Code: 201, Successful: true},
		ClusterRoleBinding: &normalization.ClusterRoleBindingCharacteristics{
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "cluster-admin"},
		},
	}

	matched, satisfied := evaluate(event, def)
	if !matched {
		t.Fatal("matched = false, want true")
	}
	if len(satisfied) != 1 || satisfied[0].ID != "role_ref_cluster_admin" {
		t.Errorf("satisfied = %+v, want exactly [role_ref_cluster_admin]", satisfied)
	}
}

// TestEvaluate_Scenario3_NegativeControl_ModificationNotCreation is the
// AC-012 negative control: the same cluster-admin role reference reached
// by a non-create verb (standing in for a modification of an existing
// binding) must not match -- scenario 3 evaluates creation only (FR-026).
func TestEvaluate_Scenario3_NegativeControl_ModificationNotCreation(t *testing.T) {
	def := realDefinition(t, "scenario-3.yaml")
	event := normalization.Event{
		Target:    normalization.Target{Resource: "clusterrolebindings"},
		Operation: normalization.Operation{Verb: "update"},
		Outcome:   normalization.Outcome{Code: 200, Successful: true},
		ClusterRoleBinding: &normalization.ClusterRoleBindingCharacteristics{
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "cluster-admin"},
		},
	}

	if matched, _ := evaluate(event, def); matched {
		t.Error("matched = true, want false for a non-create operation")
	}
}

func TestEvaluate_Scenario3_WrongRole_NoMatch(t *testing.T) {
	def := realDefinition(t, "scenario-3.yaml")
	event := normalization.Event{
		Target:    normalization.Target{Resource: "clusterrolebindings"},
		Operation: normalization.Operation{Verb: "create"},
		Outcome:   normalization.Outcome{Code: 201, Successful: true},
		ClusterRoleBinding: &normalization.ClusterRoleBindingCharacteristics{
			RoleRef: rbacv1.RoleRef{Kind: "ClusterRole", Name: "edit"},
		},
	}

	if matched, _ := evaluate(event, def); matched {
		t.Error("matched = true, want false for a non-cluster-admin role reference")
	}
}

// TestEvaluate_CrossScenario_EventOnlyMatchesItsOwnDefinition proves the
// generic engine correctly discriminates: a scenario-1 event evaluated
// against scenario-2 and scenario-3's definitions must not match either,
// and vice versa -- purely from each definition's own declared
// operation, with no scenario-specific evaluator code involved.
func TestEvaluate_CrossScenario_EventOnlyMatchesItsOwnDefinition(t *testing.T) {
	def1 := realDefinition(t, "scenario-1.yaml")
	def2 := realDefinition(t, "scenario-2.yaml")
	def3 := realDefinition(t, "scenario-3.yaml")

	execEvent := normalization.Event{
		Target:    normalization.Target{Resource: "pods", Subresource: "exec"},
		Operation: normalization.Operation{Verb: "get"},
		Exec:      &normalization.ExecCharacteristics{Stdin: true},
	}
	if matched, _ := evaluate(execEvent, def2); matched {
		t.Error("scenario-1 event matched scenario-2's definition, want false")
	}
	if matched, _ := evaluate(execEvent, def3); matched {
		t.Error("scenario-1 event matched scenario-3's definition, want false")
	}

	podEvent := normalization.Event{
		Target:      normalization.Target{Resource: "pods"},
		Operation:   normalization.Operation{Verb: "create"},
		Outcome:     normalization.Outcome{Code: 201, Successful: true},
		PodCreation: &normalization.PodCreationCharacteristics{Privileged: true},
	}
	if matched, _ := evaluate(podEvent, def1); matched {
		t.Error("scenario-2 event matched scenario-1's definition, want false")
	}
	if matched, _ := evaluate(podEvent, def3); matched {
		t.Error("scenario-2 event matched scenario-3's definition, want false")
	}
}
