import { describe, expect, it } from "vitest";
import { buildLineageLinks, resolveHighlights } from "./lineage";
import {
  fixtureIntact,
  fixtureScenario2Intact,
  fixtureScenario3Intact,
} from "@/fixtures/alert-investigation/v1";

describe("buildLineageLinks", () => {
  it("returns no links when either side is missing", () => {
    expect(buildLineageLinks(undefined, fixtureIntact.sourceEvent.rawEvent)).toEqual([]);
    expect(buildLineageLinks(fixtureIntact.normalizedEvent.event, undefined)).toEqual([]);
  });

  it("derives one link per real normalized field, including two exec fields sharing one raw source", () => {
    const links = buildLineageLinks(
      fixtureIntact.normalizedEvent.event,
      fixtureIntact.sourceEvent.rawEvent,
    );

    const byNormalizedPath = new Map(links.map((l) => [l.normalizedPath, l]));
    expect(byNormalizedPath.get("subject.username")?.rawPath).toBe("user.username");
    expect(byNormalizedPath.get("target.resource")?.rawPath).toBe("objectRef.resource");
    expect(byNormalizedPath.get("exec.stdin")?.rawPath).toBe("requestURI");
    expect(byNormalizedPath.get("exec.tty")?.rawPath).toBe("requestURI");
    expect(byNormalizedPath.get("operation.requestURI")?.rawPath).toBe("requestURI");

    // requestURI feeds three distinct normalized facts.
    const requestUriLinks = links.filter((l) => l.rawPath === "requestURI");
    expect(requestUriLinks).toHaveLength(3);
  });
});

describe("buildLineageLinks — scenario 2/3 provenance compatibility", () => {
  // lineage.ts is intentionally unmodified by the scenario 2/3 fixtures
  // (v1.ts provenance note). These two assertions are the regression
  // guard for that: podCreation and clusterRoleBinding characteristics
  // derive from requestObject, which the frontend contract types as
  // `unknown` (contract.ts), so no verified raw path exists for them yet.
  // buildLineageLinks must keep producing zero links for these fields —
  // manufacturing one would incorrectly promote them to Verified
  // provenance (UX spec §3.5) instead of the correct Partial state.

  it("produces no link for any podCreation field (scenario 2)", () => {
    const links = buildLineageLinks(
      fixtureScenario2Intact.normalizedEvent.event,
      fixtureScenario2Intact.sourceEvent.rawEvent,
    );
    expect(links.some((l) => l.normalizedPath.startsWith("podCreation."))).toBe(false);
  });

  it("produces no link for any clusterRoleBinding field (scenario 3)", () => {
    const links = buildLineageLinks(
      fixtureScenario3Intact.normalizedEvent.event,
      fixtureScenario3Intact.sourceEvent.rawEvent,
    );
    expect(links.some((l) => l.normalizedPath.startsWith("clusterRoleBinding."))).toBe(false);
  });

  it("still derives the four always-present core links for scenario 2 and 3 (subject, verb, requestURI, requestTime)", () => {
    for (const fixture of [fixtureScenario2Intact, fixtureScenario3Intact]) {
      const links = buildLineageLinks(fixture.normalizedEvent.event, fixture.sourceEvent.rawEvent);
      const normalizedPaths = links.map((l) => l.normalizedPath);
      expect(normalizedPaths).toEqual(
        expect.arrayContaining([
          "subject.username",
          "operation.verb",
          "operation.requestURI",
          "requestTime",
        ]),
      );
    }
  });
});

describe("resolveHighlights", () => {
  const links = buildLineageLinks(
    fixtureIntact.normalizedEvent.event,
    fixtureIntact.sourceEvent.rawEvent,
  );

  it("returns empty sets when nothing is selected", () => {
    const resolved = resolveHighlights(links, null);
    expect(resolved.rawPaths.size).toBe(0);
    expect(resolved.normalizedPaths.size).toBe(0);
  });

  it("selecting a normalized field highlights its single raw source", () => {
    const resolved = resolveHighlights(links, { side: "normalized", path: "exec.tty" });
    expect(resolved.rawPaths).toEqual(new Set(["requestURI"]));
    expect(resolved.normalizedPaths).toEqual(new Set(["exec.tty"]));
  });

  it("selecting the shared raw field highlights every normalized fact it feeds", () => {
    const resolved = resolveHighlights(links, { side: "raw", path: "requestURI" });
    expect(resolved.normalizedPaths).toEqual(
      new Set(["operation.requestURI", "exec.stdin", "exec.tty"]),
    );
  });
});
