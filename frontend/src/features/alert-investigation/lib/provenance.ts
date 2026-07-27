import type { AlertInvestigationResponse, Characteristic } from "@/types/contract";
import { buildLineageLinks } from "./lineage";

/**
 * The provenance-state layer (UX spec §3.5): for one satisfied
 * characteristic, whether its raw-to-normalized derivation can be shown as
 * a concrete, structurally verified mapping (Verified), only as a known
 * condition and observed fact with no guaranteed raw path (Partial), or
 * not at all because a required artifact is unavailable (Unavailable).
 * No raw field path, value, transformation, or source mapping is ever
 * invented to fill a state the data does not support — every field a
 * "verified" record carries traces to a real `lineage.ts` link, and every
 * field a "partial" record carries traces to the definition/match-reason
 * data alone.
 */

export type ProvenanceState =
  | {
      kind: "verified";
      rawPath: string;
      rawValue: string;
      /** The exact substring of `rawValue` this fact was parsed from, when
       *  the transformation rule's own documented locator (a named query
       *  parameter — see `CHARACTERISTIC_QUERY_PARAM` below) is present
       *  verbatim in `rawValue`. Absent, never guessed, when it cannot be
       *  located this way — the canvas must never mark a substring it
       *  cannot honestly point to. */
      rawHighlight?: string;
      behavior: string;
      normalizedPath: string;
      normalizedValue: string;
      condition: string;
    }
  | {
      kind: "partial";
      normalizedPath: string;
      normalizedValue: string;
      condition: string;
      limitation: string;
    }
  | {
      kind: "unavailable";
      missingArtifact: "source-event" | "normalized-event";
      condition: string;
      explanation: string;
    };

/**
 * Maps each real, currently-declared characteristic id
 * (definitions/scenario-{1,2,3}.yaml) to the real, already-typed
 * normalized-event field path it corresponds to (contract.ts). This is
 * data, not detection logic: a future scenario's characteristic needs a
 * one-line addition here, never a structural change to the functions that
 * use it. `role_ref_cluster_admin`'s real check is a compound fact
 * (roleRef.kind === "ClusterRole" && roleRef.name === "cluster-admin" —
 * internal/detection/evaluate.go `characteristicSet`); `roleRef.name` is
 * its single most representative leaf field for a provenance record — the
 * full roleRef remains inspectable via the Normalized Event artifact.
 */
const CHARACTERISTIC_NORMALIZED_PATH: Record<string, string> = {
  stdin_streaming: "exec.stdin",
  tty_allocation: "exec.tty",
  privileged_container: "podCreation.privileged",
  host_network: "podCreation.hostNetwork",
  host_pid: "podCreation.hostPID",
  host_ipc: "podCreation.hostIPC",
  host_path_volume: "podCreation.hostPathVolume",
  role_ref_cluster_admin: "clusterRoleBinding.roleRef.name",
};

/**
 * The literal query-parameter name each requestURI-derived characteristic
 * was parsed from (internal/normalization/normalization.go
 * `execCharacteristics`, restated verbatim by the ADR-0003 Q1 reference
 * already carried in `buildLineageLinks`' own label text above). Exists so
 * the canvas can mark the exact substring of the raw requestURI a Verified
 * fact came from — never a guess: a characteristic absent from this table
 * (every current or future requestObject-derived one) simply never gets a
 * `rawHighlight`, and one present here only gets one when that literal
 * `param=value` text is actually found in the raw string.
 */
const CHARACTERISTIC_QUERY_PARAM: Record<string, string> = {
  stdin_streaming: "stdin",
  tty_allocation: "tty",
};

function findRawHighlight(rawValue: string, characteristicId: string, normalizedValue: string): string | undefined {
  const param = CHARACTERISTIC_QUERY_PARAM[characteristicId];
  if (!param) return undefined;
  const marker = `${param}=${normalizedValue}`;
  return rawValue.includes(marker) ? marker : undefined;
}

function readPath(obj: unknown, path: string): unknown {
  return path.split(".").reduce<unknown>((acc, key) => {
    if (acc && typeof acc === "object" && key in (acc as Record<string, unknown>)) {
      return (acc as Record<string, unknown>)[key];
    }
    return undefined;
  }, obj);
}

function formatValue(value: unknown): string {
  if (typeof value === "boolean") return value ? "true" : "false";
  if (value === undefined) return "—";
  return String(value);
}

/**
 * Derives one matched characteristic's provenance state. Deterministic:
 * given the same characteristic and response, always returns the same
 * result, and never mutates either argument.
 */
export function deriveProvenanceState(
  characteristic: Characteristic,
  data: AlertInvestigationResponse,
): ProvenanceState {
  if (!data.sourceEvent.available) {
    return {
      kind: "unavailable",
      missingArtifact: "source-event",
      condition: characteristic.description,
      explanation: "The source event is unavailable, so this fact's raw origin cannot be inspected.",
    };
  }
  if (!data.normalizedEvent.available) {
    return {
      kind: "unavailable",
      missingArtifact: "normalized-event",
      condition: characteristic.description,
      explanation:
        "The normalized event is unavailable, so this fact's raw origin cannot be inspected.",
    };
  }

  const normalizedPath = CHARACTERISTIC_NORMALIZED_PATH[characteristic.id];
  const normalizedValue = formatValue(
    normalizedPath ? readPath(data.normalizedEvent.event, normalizedPath) : undefined,
  );

  if (normalizedPath) {
    const links = buildLineageLinks(data.normalizedEvent.event, data.sourceEvent.rawEvent);
    const link = links.find((l) => l.normalizedPath === normalizedPath);
    if (link) {
      const rawValue = formatValue(readPath(data.sourceEvent.rawEvent, link.rawPath));
      return {
        kind: "verified",
        rawPath: link.rawPath,
        rawValue,
        rawHighlight: findRawHighlight(rawValue, characteristic.id, normalizedValue),
        behavior: link.label,
        normalizedPath,
        normalizedValue,
        condition: characteristic.description,
      };
    }
  }

  return {
    kind: "partial",
    normalizedPath: normalizedPath ?? characteristic.id,
    normalizedValue,
    condition: characteristic.description,
    limitation: normalizedPath
      ? "This field's source location cannot be structurally identified in the current raw-event contract."
      : "This characteristic has no known normalized-field mapping in the current implementation.",
  };
}
