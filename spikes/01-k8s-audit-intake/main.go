package main

import (
	"encoding/json"
	"fmt"
	"os"

	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
)

func rawObjectToMap(raw []byte) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return map[string]interface{}{"_unmarshalError": err.Error()}
	}
	return m
}

func mapEvent(ev *auditv1.Event) CanonicalSubmission {
	sub := CanonicalSubmission{
		AuditID:      string(ev.AuditID),
		Stage:        string(ev.Stage),
		Verb:         ev.Verb,
		RequestURI:   ev.RequestURI,
		UserName:     ev.User.Username,
		SourceIPs:    ev.SourceIPs,
	}
	if ev.ObjectRef != nil {
		sub.ResourceKind = ev.ObjectRef.Resource
		sub.Subresource = ev.ObjectRef.Subresource
		sub.ObjectName = ev.ObjectRef.Name
		sub.ObjectNS = ev.ObjectRef.Namespace
	}
	if ev.ResponseStatus != nil {
		sub.ResponseCode = ev.ResponseStatus.Code
	}

	var reqObj map[string]interface{}
	if ev.RequestObject != nil {
		reqObj = rawObjectToMap(ev.RequestObject.Raw)
	}
	var respObj map[string]interface{}
	if ev.ResponseObject != nil {
		respObj = rawObjectToMap(ev.ResponseObject.Raw)
	}

	// Retain the whole original event verbatim as the raw source event
	// reference (refinement 2) — nothing FR-002/FR-007 might need is
	// dropped just because the canonical model above doesn't carry it.
	rawBytes, _ := json.Marshal(ev)
	var rawMap map[string]interface{}
	_ = json.Unmarshal(rawBytes, &rawMap)
	sub.RawSourceEvent = rawMap

	switch {
	case sub.ResourceKind == "pods" && sub.Subresource == "exec":
		sub.Exec = extractExecSignals(sub.RequestURI)
	case sub.ResourceKind == "pods" && sub.Verb == "create":
		sub.HighRiskPod = extractHighRiskPodSignals(reqObj)
	case sub.ResourceKind == "clusterrolebindings":
		sub.ClusterRoleBindingGrant = extractCRBSignals(sub.Verb, reqObj, respObj)
	}

	return sub
}

func extractExecSignals(requestURI string) *ExecSignals {
	// tty/stdin are query parameters on the exec subresource requestURI,
	// not fields inside a requestObject (exec has no request body).
	sig := &ExecSignals{}
	sig.TTYRequested = containsQueryFlag(requestURI, "tty=true")
	sig.StdinRequested = containsQueryFlag(requestURI, "stdin=true")
	return sig
}

func containsQueryFlag(uri, flag string) bool {
	for i := 0; i+len(flag) <= len(uri); i++ {
		if uri[i:i+len(flag)] == flag {
			return true
		}
	}
	return false
}

func extractHighRiskPodSignals(reqObj map[string]interface{}) *HighRiskPodSignals {
	if reqObj == nil {
		return nil
	}
	spec, _ := reqObj["spec"].(map[string]interface{})
	if spec == nil {
		return nil
	}
	sig := &HighRiskPodSignals{}
	sig.HostNetwork, _ = spec["hostNetwork"].(bool)
	sig.HostPID, _ = spec["hostPID"].(bool)
	sig.HostIPC, _ = spec["hostIPC"].(bool)

	if containers, ok := spec["containers"].([]interface{}); ok {
		for _, c := range containers {
			cm, _ := c.(map[string]interface{})
			if sc, ok := cm["securityContext"].(map[string]interface{}); ok {
				if priv, ok := sc["privileged"].(bool); ok && priv {
					sig.Privileged = true
				}
			}
		}
	}
	if volumes, ok := spec["volumes"].([]interface{}); ok {
		for _, v := range volumes {
			vm, _ := v.(map[string]interface{})
			if hp, ok := vm["hostPath"].(map[string]interface{}); ok {
				if p, ok := hp["path"].(string); ok {
					sig.HostPathVolumes = append(sig.HostPathVolumes, p)
				}
			}
		}
	}
	return sig
}

func subjectStrings(subjects []interface{}) []string {
	var out []string
	for _, s := range subjects {
		sm, _ := s.(map[string]interface{})
		out = append(out, fmt.Sprintf("%v:%v", sm["kind"], sm["name"]))
	}
	return out
}

func extractCRBSignals(verb string, reqObj, respObj map[string]interface{}) *ClusterRoleBindingSignals {
	sig := &ClusterRoleBindingSignals{}

	if respObj != nil {
		if roleRef, ok := respObj["roleRef"].(map[string]interface{}); ok {
			sig.RoleRefName, _ = roleRef["name"].(string)
		}
		if subjects, ok := respObj["subjects"].([]interface{}); ok {
			sig.ResultingSubjects = subjectStrings(subjects)
		}
	}

	switch verb {
	case "create":
		// Creation: every subject present is provably part of the grant
		// made by this event, since there is no prior state to compare
		// against — the whole object IS the new state.
		sig.SubjectAdditionProvable = true
		sig.ProvabilityReason = "verb=create: the whole object is new, so every listed subject is provably part of this grant"

	case "patch":
		// Determine whether the request body is an RFC 6902 JSON Patch
		// with an explicit "add" op — the only observed case where a
		// single event's requestObject proves a subject was newly added
		// rather than merely resubmitted. See FINDINGS.md finding CRB-1.
		if isJSONPatchAdd(reqObj) {
			sig.SubjectAdditionProvable = true
			sig.ProvabilityReason = "verb=patch with an explicit RFC 6902 JSON Patch \"add\" operation on /subjects: requestObject itself proves an addition was requested"
		} else {
			sig.SubjectAdditionProvable = false
			sig.ProvabilityReason = "verb=patch (apply/merge-style): requestObject/responseObject carry only the resulting full subjects list, not a diff — cannot prove a subject is NEW without the prior state of this ClusterRoleBinding"
		}

	case "update":
		sig.SubjectAdditionProvable = false
		sig.ProvabilityReason = "verb=update: requestObject/responseObject carry only the resulting full subjects list, not a diff — cannot prove a subject is NEW without the prior state of this ClusterRoleBinding"

	default:
		sig.ProvabilityReason = fmt.Sprintf("verb=%s not analyzed by this spike", verb)
	}

	return sig
}

// isJSONPatchAdd reports whether a captured requestObject (which, for
// PATCH requests, holds the raw patch body rather than a Kubernetes
// object) is a JSON Patch array containing an "add" operation.
// Note: this check operates on the map produced by decoding requestObject
// as an object; a genuine JSON Patch body is actually a JSON *array*, so
// in practice this spike inspects the raw bytes directly (see main()).
func isJSONPatchAdd(_ map[string]interface{}) bool {
	return false
}

func main() {
	data, err := os.ReadFile("fixtures/webhook-eventlist.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, "read eventlist:", err)
		os.Exit(1)
	}

	var list auditv1.EventList
	if err := json.Unmarshal(data, &list); err != nil {
		fmt.Fprintln(os.Stderr, "unmarshal EventList with official audit/v1 types:", err)
		os.Exit(1)
	}
	fmt.Printf("Parsed EventList: kind=%s apiVersion=%s items=%d\n\n", list.Kind, list.APIVersion, len(list.Items))

	var results []CanonicalSubmission
	for i := range list.Items {
		ev := &list.Items[i]

		// Patch requestObject bodies are JSON *arrays* (RFC 6902), which
		// runtime.Unknown/json.Unmarshal into our map-based helper above
		// can't represent — detect that shape directly from the raw bytes.
		var isPatchAdd bool
		if ev.RequestObject != nil {
			var arr []map[string]interface{}
			if err := json.Unmarshal(ev.RequestObject.Raw, &arr); err == nil {
				for _, op := range arr {
					if op["op"] == "add" {
						isPatchAdd = true
					}
				}
			}
		}

		sub := mapEvent(ev)
		if sub.ClusterRoleBindingGrant != nil && ev.Verb == "patch" && isPatchAdd {
			sub.ClusterRoleBindingGrant.SubjectAdditionProvable = true
			sub.ClusterRoleBindingGrant.ProvabilityReason = "verb=patch with an explicit RFC 6902 JSON Patch \"add\" operation on /subjects: requestObject itself proves an addition was requested"
		}

		results = append(results, sub)

		fmt.Printf("--- event %d ---\n", i)
		fmt.Printf("auditID=%s stage=%s verb=%s resource=%s subresource=%s name=%s\n",
			sub.AuditID, sub.Stage, sub.Verb, sub.ResourceKind, sub.Subresource, sub.ObjectName)
		if sub.Exec != nil {
			fmt.Printf("  exec signals: tty=%v stdin=%v\n", sub.Exec.TTYRequested, sub.Exec.StdinRequested)
		}
		if sub.HighRiskPod != nil {
			fmt.Printf("  high-risk pod signals: privileged=%v hostNetwork=%v hostPID=%v hostIPC=%v hostPathVolumes=%v\n",
				sub.HighRiskPod.Privileged, sub.HighRiskPod.HostNetwork, sub.HighRiskPod.HostPID,
				sub.HighRiskPod.HostIPC, sub.HighRiskPod.HostPathVolumes)
		}
		if sub.ClusterRoleBindingGrant != nil {
			fmt.Printf("  CRB signals: roleRef=%s subjects=%v additionProvable=%v reason=%s\n",
				sub.ClusterRoleBindingGrant.RoleRefName, sub.ClusterRoleBindingGrant.ResultingSubjects,
				sub.ClusterRoleBindingGrant.SubjectAdditionProvable, sub.ClusterRoleBindingGrant.ProvabilityReason)
		}
		fmt.Println()
	}

	out, _ := json.MarshalIndent(results, "", "  ")
	if err := os.WriteFile("fixtures/mapped-canonical-submissions.json", out, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "write mapped output:", err)
		os.Exit(1)
	}
	fmt.Println("Wrote fixtures/mapped-canonical-submissions.json")
}
