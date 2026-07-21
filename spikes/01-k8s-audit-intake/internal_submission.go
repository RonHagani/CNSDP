package main

// CanonicalSubmission is the spike's minimal stand-in for the eventual
// canonical internal submission model referenced by the dual-layer intake
// decision. It carries only what is needed to check FR-002/FR-007 field
// presence and scenario-relevant semantics for the three approved v0.1
// detection scenarios — it is not the product's real internal model.
type CanonicalSubmission struct {
	AuditID       string `json:"auditId"`
	Stage         string `json:"stage"`
	Verb          string `json:"verb"`
	RequestURI    string `json:"requestUri"`
	ResourceKind  string `json:"resourceKind"`
	Subresource   string `json:"subresource,omitempty"`
	ObjectName    string `json:"objectName"`
	ObjectNS      string `json:"objectNamespace,omitempty"`
	UserName      string `json:"userName"`
	SourceIPs     []string `json:"sourceIPs,omitempty"`
	ResponseCode  int32  `json:"responseCode"`

	// RawSourceEvent retains the entire originally received audit Event,
	// verbatim, as required by refinement 2: fields the canonical model
	// does not itself carry must not be lost as evidence.
	RawSourceEvent map[string]interface{} `json:"rawSourceEvent"`

	// Scenario-specific extracted signals (nil/zero when not applicable
	// or not determinable from this single event).
	Exec *ExecSignals `json:"exec,omitempty"`
	HighRiskPod *HighRiskPodSignals `json:"highRiskPod,omitempty"`
	ClusterRoleBindingGrant *ClusterRoleBindingSignals `json:"clusterRoleBindingGrant,omitempty"`
}

type ExecSignals struct {
	TTYRequested   bool `json:"ttyRequested"`
	StdinRequested bool `json:"stdinRequested"`
	Container      string `json:"container,omitempty"`
}

type HighRiskPodSignals struct {
	Privileged      bool     `json:"privileged"`
	HostNetwork     bool     `json:"hostNetwork"`
	HostPID         bool     `json:"hostPID"`
	HostIPC         bool     `json:"hostIPC"`
	HostPathVolumes []string `json:"hostPathVolumes,omitempty"`
}

// ClusterRoleBindingSignals reflects what THIS single event alone can and
// cannot establish about a subject grant. See finding CRB-1 in FINDINGS.md:
// SubjectAdditionProvable is only true when the request body itself is an
// explicit RFC 6902 JSON Patch "add" operation. For full-object apply and
// merge-patch requests, the platform cannot determine newness from this
// event alone — ResultingSubjects is the best available signal, and
// SubjectAdditionProvable is false, with a reason recorded.
type ClusterRoleBindingSignals struct {
	RoleRefName              string   `json:"roleRefName"`
	ResultingSubjects         []string `json:"resultingSubjects"`
	SubjectAdditionProvable   bool     `json:"subjectAdditionProvable"`
	ProvabilityReason         string   `json:"provabilityReason"`
}
