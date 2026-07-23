import { describe, expect, it } from "vitest";
import { buildLineageLinks, resolveHighlights } from "./lineage";
import { fixtureIntact } from "@/fixtures/alert-investigation/v1";

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
