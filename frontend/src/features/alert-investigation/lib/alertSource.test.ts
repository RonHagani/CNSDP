import { describe, expect, it } from "vitest";
import { AlertFetchError, fetchAlertInvestigation } from "./alertSource";

describe("fetchAlertInvestigation", () => {
  it("resolves the intact fixture for id 1", async () => {
    const result = await fetchAlertInvestigation("1");
    expect(result.alertId).toBe(1);
    expect(result.traceability.intact).toBe(true);
  });

  it("resolves the partial-availability fixture for id 2", async () => {
    const result = await fetchAlertInvestigation("2");
    expect(result.detectionDefinition.available).toBe(false);
  });

  it("resolves the broken-traceability fixture for id 3", async () => {
    const result = await fetchAlertInvestigation("3");
    expect(result.traceability.intact).toBe(false);
    expect(result.traceability.failedLink).toBe("raw_event_sha256");
  });

  it("rejects with a not-found AlertFetchError for an unknown id", async () => {
    await expect(fetchAlertInvestigation("999")).rejects.toMatchObject({
      name: "AlertFetchError",
      kind: "not-found",
    });
  });

  it("rejects with an unauthorized AlertFetchError under the unauthorized demo scenario", async () => {
    const error = await fetchAlertInvestigation("1", "unauthorized").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(AlertFetchError);
    expect((error as AlertFetchError).kind).toBe("unauthorized");
  });

  it("rejects with an unavailable AlertFetchError under the unavailable demo scenario", async () => {
    const error = await fetchAlertInvestigation("1", "unavailable").catch((e: unknown) => e);
    expect(error).toBeInstanceOf(AlertFetchError);
    expect((error as AlertFetchError).kind).toBe("unavailable");
  });
});
