import { describe, expect, it } from "vitest";
import { deriveProvenanceState } from "./provenance";
import {
  fixtureIntact,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";
import type { AlertInvestigationResponse, Characteristic } from "@/types/contract";

describe("deriveProvenanceState — Verified (scenario 1, requestURI-derived)", () => {
  it("stdin_streaming resolves to a concrete raw path/value and behavior", () => {
    const characteristic: Characteristic = {
      id: "stdin_streaming",
      description: "The exec request enables standard-input streaming.",
    };
    const state = deriveProvenanceState(characteristic, fixtureIntact);
    expect(state.kind).toBe("verified");
    if (state.kind === "verified") {
      expect(state.normalizedPath).toBe("exec.stdin");
      expect(state.normalizedValue).toBe("true");
      expect(state.rawPath).toBe("requestURI");
      expect(state.rawValue).toContain("stdin=true");
      expect(state.behavior.length).toBeGreaterThan(0);
      expect(state.condition).toBe(characteristic.description);
    }
  });

  it("tty_allocation also resolves to Verified", () => {
    const characteristic: Characteristic = {
      id: "tty_allocation",
      description: "The exec request requests interactive terminal (TTY) allocation.",
    };
    const state = deriveProvenanceState(characteristic, fixtureIntact);
    expect(state.kind).toBe("verified");
    if (state.kind === "verified") {
      expect(state.normalizedPath).toBe("exec.tty");
      expect(state.normalizedValue).toBe("true");
    }
  });
});

describe("deriveProvenanceState — Partial (requestObject-derived)", () => {
  it("scenario 2's privileged_container is Partial, not Verified", () => {
    const characteristic: Characteristic = {
      id: "privileged_container",
      description: "A container in the Pod requests privileged mode.",
    };
    const state = deriveProvenanceState(characteristic, fixtureScenario2Intact);
    expect(state.kind).toBe("partial");
    if (state.kind === "partial") {
      expect(state.normalizedPath).toBe("podCreation.privileged");
      expect(state.normalizedValue).toBe("true");
      expect(state.condition).toBe(characteristic.description);
      expect(state.limitation.length).toBeGreaterThan(0);
    }
  });

  it("every satisfied scenario 2 characteristic is Partial", () => {
    const satisfied = fixtureScenario2Intact.detectionResult.matchReason!.satisfiedCharacteristics;
    for (const c of satisfied) {
      expect(deriveProvenanceState(c, fixtureScenario2Intact).kind).toBe("partial");
    }
  });

  it("scenario 3's role_ref_cluster_admin is Partial, not Verified", () => {
    const characteristic: Characteristic = {
      id: "role_ref_cluster_admin",
      description: "The ClusterRoleBinding's role reference is the cluster-admin ClusterRole.",
    };
    const state = deriveProvenanceState(characteristic, fixtureScenario3Intact);
    expect(state.kind).toBe("partial");
    if (state.kind === "partial") {
      expect(state.normalizedPath).toBe("clusterRoleBinding.roleRef.name");
      expect(state.normalizedValue).toBe("cluster-admin");
    }
  });

  it("a Partial record never fabricates a raw path or transformation behavior", () => {
    const state = deriveProvenanceState(
      { id: "host_network", description: "The Pod uses the host network." },
      fixtureScenario2Intact,
    );
    expect(state).not.toHaveProperty("rawPath");
    expect(state).not.toHaveProperty("rawValue");
    expect(state).not.toHaveProperty("behavior");
  });
});

describe("deriveProvenanceState — Unavailable", () => {
  const base = fixtureIntact;
  const characteristic: Characteristic = {
    id: "stdin_streaming",
    description: "The exec request enables standard-input streaming.",
  };

  it("source event unavailable", () => {
    const data: AlertInvestigationResponse = { ...base, sourceEvent: { available: false } };
    const state = deriveProvenanceState(characteristic, data);
    expect(state.kind).toBe("unavailable");
    if (state.kind === "unavailable") {
      expect(state.missingArtifact).toBe("source-event");
    }
  });

  it("normalized event unavailable", () => {
    const data: AlertInvestigationResponse = { ...base, normalizedEvent: { available: false } };
    const state = deriveProvenanceState(characteristic, data);
    expect(state.kind).toBe("unavailable");
    if (state.kind === "unavailable") {
      expect(state.missingArtifact).toBe("normalized-event");
    }
  });

  it("source-event unavailability takes precedence when both are unavailable", () => {
    const data: AlertInvestigationResponse = {
      ...base,
      sourceEvent: { available: false },
      normalizedEvent: { available: false },
    };
    const state = deriveProvenanceState(characteristic, data);
    expect(state.kind).toBe("unavailable");
    if (state.kind === "unavailable") {
      expect(state.missingArtifact).toBe("source-event");
    }
  });

  it("Unavailable provenance is distinct from broken traceability — an intact-traceability fixture can still report Unavailable when an artifact is missing", () => {
    const data: AlertInvestigationResponse = { ...base, sourceEvent: { available: false } };
    expect(data.traceability.intact).toBe(true);
    expect(deriveProvenanceState(characteristic, data).kind).toBe("unavailable");
  });
});

describe("deriveProvenanceState — no fabricated links", () => {
  it("an unrecognized characteristic id degrades to Partial, never Verified", () => {
    const state = deriveProvenanceState(
      { id: "not_a_real_characteristic", description: "Not a real declared characteristic." },
      fixtureIntact,
    );
    expect(state.kind).toBe("partial");
  });
});
