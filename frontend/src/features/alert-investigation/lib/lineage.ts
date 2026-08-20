import type { CloudTrailRawRecord, NormalizedEvent, RawAuditEvent } from "@/types/contract";
import { isCloudTrailRawEvent } from "@/types/contract";

/**
 * One raw-to-normalized field lineage link. Derived from the real
 * transformation rules in internal/normalization/normalization.go
 * (`Normalize`, `targetOf`, `outcomeOf`, `execCharacteristics`) — not an
 * invented mapping. Multiple links may share the same `rawPath` (the
 * request URI, for instance, is the source of the verbatim operation
 * field *and* the parsed exec characteristics) — selecting that raw field
 * must highlight every normalized fact it feeds.
 */
export interface LineageLink {
  normalizedPath: string;
  rawPath: string;
  label: string;
}

/**
 * Builds the real lineage links for one alert's raw/normalized event pair.
 * Only links whose raw field is actually present are returned, so this
 * degrades correctly for events lacking optional fields. Every rule below
 * describes Kubernetes' own raw-to-normalized derivation
 * (internal/normalization/normalization.go's `normalizeKubernetes` path);
 * a CloudTrail-shaped raw event (ADR-0006, `isCloudTrailRawEvent`) is
 * accepted in the type but always yields no links -- no structurally
 * verified raw path exists yet for a CloudTrail-derived field, so those
 * characteristics correctly resolve to the Partial provenance state
 * (`lib/provenance.ts`) rather than an invented one.
 */
export function buildLineageLinks(
  normalized: NormalizedEvent | undefined,
  raw: RawAuditEvent | CloudTrailRawRecord | undefined,
): LineageLink[] {
  if (!normalized || !raw || isCloudTrailRawEvent(raw)) return [];

  const links: LineageLink[] = [
    {
      normalizedPath: "subject.username",
      rawPath: "user.username",
      label: "The requesting subject is carried verbatim from user.username.",
    },
    {
      normalizedPath: "operation.verb",
      rawPath: "verb",
      label: "The operation's verb is carried verbatim from the audit event's verb.",
    },
    {
      normalizedPath: "operation.requestURI",
      rawPath: "requestURI",
      label: "The request URI is preserved verbatim as core normalized content.",
    },
    {
      normalizedPath: "requestTime",
      rawPath: "requestReceivedTimestamp",
      label: "The request time is carried verbatim from requestReceivedTimestamp.",
    },
  ];

  if (raw.objectRef) {
    if (raw.objectRef.resource) {
      links.push({
        normalizedPath: "target.resource",
        rawPath: "objectRef.resource",
        label:
          "The target resource is the API server's REST resource name from objectRef.resource.",
      });
    }
    if (raw.objectRef.name) {
      links.push({
        normalizedPath: "target.name",
        rawPath: "objectRef.name",
        label: "The target name is carried verbatim from objectRef.name.",
      });
    }
    if (raw.objectRef.namespace) {
      links.push({
        normalizedPath: "target.namespace",
        rawPath: "objectRef.namespace",
        label: "The target namespace is carried verbatim from objectRef.namespace.",
      });
    }
    if (raw.objectRef.subresource) {
      links.push({
        normalizedPath: "target.subresource",
        rawPath: "objectRef.subresource",
        label: "The target subresource is carried verbatim from objectRef.subresource.",
      });
    }
  }

  if (raw.responseStatus?.code !== undefined) {
    links.push({
      normalizedPath: "outcome.code",
      rawPath: "responseStatus.code",
      label: "The recorded outcome code is carried verbatim from responseStatus.code.",
    });
  }

  if (normalized.exec) {
    links.push({
      normalizedPath: "exec.stdin",
      rawPath: "requestURI",
      label:
        'Parsed from the requestURI\'s "stdin" query parameter (ADR-0003 Q1) — never requestObject.',
    });
    links.push({
      normalizedPath: "exec.tty",
      rawPath: "requestURI",
      label:
        'Parsed from the requestURI\'s "tty" query parameter (ADR-0003 Q1) — never requestObject.',
    });
  }

  return links;
}

export interface LineageSelection {
  side: "raw" | "normalized";
  path: string;
}

export interface ResolvedHighlight {
  rawPaths: Set<string>;
  normalizedPaths: Set<string>;
  activeLinks: LineageLink[];
}

export function resolveHighlights(
  links: LineageLink[],
  selection: LineageSelection | null,
): ResolvedHighlight {
  if (!selection) {
    return { rawPaths: new Set(), normalizedPaths: new Set(), activeLinks: [] };
  }
  const activeLinks =
    selection.side === "normalized"
      ? links.filter((l) => l.normalizedPath === selection.path)
      : links.filter((l) => l.rawPath === selection.path);

  return {
    rawPaths: new Set(activeLinks.map((l) => l.rawPath)),
    normalizedPaths: new Set(activeLinks.map((l) => l.normalizedPath)),
    activeLinks,
  };
}
