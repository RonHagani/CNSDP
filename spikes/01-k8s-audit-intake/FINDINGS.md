# Spike 1 Findings — Kubernetes audit intake

Status: complete. All data below was captured from a real, throwaway `kind`
cluster (`kindest/node:v1.34.0`) performing genuine `pods/exec`, Pod
creation, and ClusterRoleBinding create/modify operations — not hand-written
or guessed payloads. The cluster has been deleted and the node image removed;
only the captured fixtures and this analysis remain.

## Deviation from the approved spike design (disclosed, not silent)

The approved plan described capturing via the Kubernetes audit **webhook**
backend. This spike instead used the audit **log-file** backend
(`--audit-log-path`, mounted out to the host via a kind `extraMount`) to
avoid open-ended debugging of cross-container networking from a nested kind
node to a host-side HTTP listener across Docker Desktop's WSL2 layer.

This does not weaken the findings: both backends serialize the exact same
`audit.k8s.io/v1 Event` objects — the log backend writes one JSON object per
line, the webhook backend POSTs the same objects batched inside an
`EventList`. Every finding below about individual event *content* is
unaffected. To still validate the webhook **wire format** specifically, the
captured real events were wrapped into a real `{"kind":"EventList",
"apiVersion":"audit.k8s.io/v1","items":[...]}` structure
(`fixtures/webhook-eventlist.json`) and parsed as a batch using the official
`k8s.io/apiserver/pkg/apis/audit/v1.EventList` type — this succeeded without
error (see Q1 below), which is the specific thing the webhook-format check
needed to prove.

## Q1 — Do real payloads parse cleanly with the official Go audit types, and were required fields present?

**Yes**, for all three scenarios. `go run .` (`main.go`) parses
`fixtures/webhook-eventlist.json` with `k8s.io/apiserver/pkg/apis/audit/v1`
(module `k8s.io/apiserver v0.31.4`) with zero unmarshal errors across all 6
captured events, and every field FR-002/FR-007 implies as required was
located:

| Scenario | Fields checked | Result |
|---|---|---|
| `pods/exec` | `tty` and `stdin` request flags | Present — but **not** as `requestObject` fields (exec has no request body). They are **query parameters on `requestURI`** (`...&stdin=true&tty=true`). A parser that only inspects `requestObject`/`responseObject` will miss them entirely. |
| High-risk Pod creation | `spec.hostNetwork`, `spec.hostPID`, `spec.hostIPC`, `spec.containers[].securityContext.privileged`, `spec.volumes[].hostPath.path` | All present, at exactly the paths FR-002/FR-007 assume, inside `requestObject`. |
| ClusterRoleBinding | `roleRef.name`, `subjects[]` | Present in `requestObject`/`responseObject` for `create`; see Q3 for the modification case, which is more complicated. |

**Correction to my own prior assumption**: the captured exec event's `verb`
field is `"get"`, not `"connect"` as commonly assumed in detection write-ups
and as I stated in the earlier evaluation's walking-skeleton description.
Confirmed genuine (not a capture artifact) via `responseStatus.code: 101`
("Switching Protocols") present identically across all three audit stages
(`RequestReceived`, `ResponseStarted`, `ResponseComplete`) — a real,
successful, TTY-allocated exec upgrade. **Detection logic for scenario 1
must not key on `verb == "connect"`.** It should key on `objectRef.resource
== "pods"`, `objectRef.subresource == "exec"`, and the `tty=true` (and
optionally `stdin=true`) query flags in `requestURI`. This should be
re-checked against whatever Kubernetes version the reference environment
and any real target cluster actually runs, since subresource verb labeling
has changed across Kubernetes versions historically — this spike verified
one real data point (v1.34), not a cross-version guarantee.

## Q2 — Does `requestObject`/`responseObject` appear at every audit stage?

**No.** For the same logical request, content differs sharply by stage:

- `RequestReceived`: **no** `requestObject`, **no** `responseObject`, no
  `responseStatus` — the request body has not yet been recorded at
  `RequestResponse` policy level in this configuration.
- `ResponseStarted`: appears only for connection-upgrade/long-running
  requests (observed for `pods/exec`); carries `responseStatus` but not yet
  the full `responseObject`.
- `ResponseComplete`: the only stage that reliably carries both
  `requestObject` and `responseObject`.

Ordinary synchronous requests (`create`, `patch` on Pods/ClusterRoleBindings)
produced exactly **two** stages (`RequestReceived`, `ResponseComplete`);
`pods/exec` (a connect/upgrade request) produced **three**
(`RequestReceived`, `ResponseStarted`, `ResponseComplete`). **Any intake
adapter that consumes only `RequestReceived`-stage events will have neither
`requestObject` nor `responseObject` to work with** — validation/detection
logic that needs object content must consume `ResponseComplete` (or, for
exec specifically, may be able to act on `RequestReceived`'s `requestURI`
alone, since the tty/stdin signal lives in the URI, not the body).

## Q3 — Can a single audit event prove a ClusterRoleBinding subject was newly added? **Primary uncertainty — result: not always, and not by the most common real workflow.**

Tested three real, distinct ways of adding `mallory@example.com` as a second
subject to an existing `cluster-admin` ClusterRoleBinding that already had
one subject (`alice@example.com`):

| Variant | Real command | `verb` | `requestObject` content | Proves addition? |
|---|---|---|---|---|
| A — full update via `kubectl apply` | `kubectl apply -f crb-a-updated.yaml` | `patch` (not `update`/`put` — see below) | Full resulting `subjects` list (both users), plus a `last-applied-configuration` annotation | **No** |
| B — merge patch, full array resupplied | `kubectl patch --type=merge -p '{"subjects":[...]}'` | `patch` | Just `{"subjects":[...both users...]}` — the full desired list, since list fields have no merge key | **No** |
| C — explicit JSON Patch add | `kubectl patch --type=json -p '[{"op":"add","path":"/subjects/-","value":{...mallory...}}]'` | `patch` | The literal patch operation array, `op: "add"` | **Yes** |

**Finding CRB-1 (the core result)**: only variant C's `requestObject` is
self-proving. Variants A and B — `kubectl apply` and merge-patch, which
together represent the overwhelmingly common real-world way RBAC changes
are actually made (GitOps reconciliation, Helm, `kubectl edit`, most
RBAC-management tooling) — produce a `requestObject`/`responseObject` pair
that is **structurally indistinguishable** between "this subject is newly
added" and "this subject was already present and got resubmitted as part of
an unrelated change." Determining novelty in those cases requires comparing
against the ClusterRoleBinding's **prior** state, which is not present in a
single audit event.

**Corrected assumption**: `kubectl apply`, popularly thought of as "full
object replacement," is implemented as a `PATCH` request (verb `patch`) in
this captured Kubernetes version — never a `PUT`/`update` — because client-side
apply computes and sends a three-way-merge-style patch carrying the desired
full object plus a `last-applied-configuration` annotation. **There is no
observed real-world `verb: "update"` case for this scenario**; the spike's
originally planned "full update" vs. "patch" comparison collapses in
practice to "two different shapes of patch," both non-proving, versus one
explicit-intent patch shape that does prove addition.

**This is a genuine mismatch with the current requirements baseline, not a
gap the spike can silently work around.** As FR-023 and the glossary's
*Detection result* entry currently frame detection as individually
evaluating one normalized event with no cross-event state, and as PD-04
scope decision 7 speaks of "provable... addition," the requirement as
written is **not achievable for the two most common real modification
paths** using only the single triggering event. Reconciling this requires
an explicit decision — not made by this spike — between at minimum:

1. Narrow the requirement to only the case that is provable from a single
   event (explicit JSON-Patch adds) — a real but materially incomplete
   detection surface for this scenario.
2. Accept detection only at `create` time for cluster-admin bindings, and
   treat *any* `patch`/`update` to an existing cluster-admin binding's
   `subjects` as scenario-relevant regardless of provable novelty (trading
   false positives — resubmission of an unchanged subject list — for not
   missing real additions).
3. Introduce statefulness: have the platform retain the last-known
   `subjects` list per ClusterRoleBinding (from the normalized
   representation of the previous event affecting that object) and diff
   against it. This is a real architectural change — detection evaluation
   would no longer be a pure function of one event in isolation for this
   scenario — and should be raised as an explicit requirements/architecture
   question rather than assumed away.

This finding is recorded as a requirement-reconciliation item, not resolved
here.

**Final product decision (adopted after this spike):** for v0.1, Detection
Scenario 3 covers only successful creation of a ClusterRoleBinding
referencing the cluster-admin ClusterRole. Detection of a subject being
added to an already-existing ClusterRoleBinding is deferred to a future
release, because the common real-world modification techniques tested here
(`kubectl apply` and merge-patch) do not provide sufficient single-event,
stateless proof of the previous-versus-new subject set. The narrow
JSON-Patch-add case that *does* self-prove an addition is not retained as a
special-case detection branch — it is a rare, atypical way of making this
change in practice, and keeping a branch that only fires for it would not
meaningfully cover the real risk while adding a detection path most real
modifications would never take. This decision is reflected in `docs/scope.md`
(PD-04, Scenario 3 and "Deferred to later releases"), `docs/
functional-requirements.md` (PD-05, FR-018 and FR-026), and `docs/
acceptance-criteria.md` (PD-07, AC-012).

## Files produced

- `kind-config.yaml`, `audit-policy.yaml` — throwaway cluster config (audit
  log-file backend, `RequestResponse` level for pods/pods-exec/CRBs)
- `high-risk-pod.yaml`, `crb-*-initial.yaml`, `crb-a-updated.yaml` — the
  real manifests applied to generate the fixtures
- `audit-logs/audit.log` — the complete raw audit log from the live cluster
  (3,717 lines, 4,253,096 bytes; retained for traceability of every fixture
  back to its original capture)
- `fixtures/exec-tty-true.jsonl`, `fixtures/pod-create-highrisk.jsonl`,
  `fixtures/crb-create-events.jsonl`,
  `fixtures/crb-subject-addition-variants.jsonl` — isolated single-purpose
  real events per scenario
- `fixtures/webhook-eventlist.json` — the same real events wrapped in a
  genuine `EventList` webhook-wire-format envelope
- `internal_submission.go`, `main.go` — the canonical-model mapping,
  written against the official `k8s.io/apiserver/pkg/apis/audit/v1` types
- `fixtures/mapped-canonical-submissions.json` — the mapping's output over
  all 6 real events

## What this spike does not resolve

- Whether `verb == "get"` for exec holds across other Kubernetes versions —
  verified for v1.34.0 only.
- Live webhook network delivery end-to-end (deliberately substituted; see
  deviation note above) — the wire-format shape was validated, not live
  HTTP delivery mechanics (retries, TLS, backpressure).
- `pods/exec` without `-it` (non-interactive exec), and non-`kubectl`
  clients constructing raw exec requests — not exercised.

## Environment cleanup performed

- `kind delete cluster --name spike1-audit` — done, containers removed.
- `kindest/node` image — force-removed from Docker.
- Go toolchain and Docker Desktop left installed/running — durable project
  dependencies under the locked architecture, not spike-specific.
