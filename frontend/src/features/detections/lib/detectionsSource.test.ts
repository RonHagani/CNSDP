import { describe, expect, it, vi } from "vitest";
import { mockJsonResponse, stubFetchRoutes } from "@/test/mockFetch";
import { DetectionsFetchError, fetchDetections } from "./detectionsSource";

const scenario1Detection = {
  scenario: "scenario-1",
  name: "Interactive container exec request",
  description: "Detection of a Kubernetes API request to the pods/exec subresource.",
  revision: "a1b2c3d4e5f60718293a4b5c6d7e8f9012345678901234567890abcdef01234",
  conditions: {
    operation: { resource: "pods", subresource: "exec" },
    requires_any: [
      { id: "stdin_streaming", description: "The exec request enables standard-input streaming." },
      { id: "tty_allocation", description: "The exec request requests interactive terminal (TTY) allocation." },
    ],
  },
};

describe("fetchDetections", () => {
  it("requests GET /v1/detections through the same-origin /api proxy prefix", async () => {
    const fetchMock = stubFetchRoutes({
      "/v1/detections": () => mockJsonResponse(200, { detections: [scenario1Detection], total: 1 }),
    });
    await fetchDetections();
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/detections", expect.anything());
  });

  it("resolves the backend's detections and total exactly as returned", async () => {
    stubFetchRoutes({
      "/v1/detections": () => mockJsonResponse(200, { detections: [scenario1Detection], total: 1 }),
    });
    const result = await fetchDetections();
    expect(result).toEqual({ detections: [scenario1Detection], total: 1 });
  });

  it("rejects with an unauthorized DetectionsFetchError for a 401", async () => {
    stubFetchRoutes({ "/v1/detections": () => mockJsonResponse(401, {}) });
    const error = await fetchDetections().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(DetectionsFetchError);
    expect((error as DetectionsFetchError).kind).toBe("unauthorized");
  });

  it("rejects with an unavailable DetectionsFetchError for a 5xx", async () => {
    stubFetchRoutes({ "/v1/detections": () => mockJsonResponse(503, {}) });
    const error = await fetchDetections().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(DetectionsFetchError);
    expect((error as DetectionsFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable DetectionsFetchError on a genuine network failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    const error = await fetchDetections().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(DetectionsFetchError);
    expect((error as DetectionsFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable DetectionsFetchError when the body does not match the contract shape", async () => {
    stubFetchRoutes({ "/v1/detections": () => mockJsonResponse(200, { unexpected: true }) });
    const error = await fetchDetections().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(DetectionsFetchError);
    expect((error as DetectionsFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable DetectionsFetchError when a detection row is missing its operation", async () => {
    stubFetchRoutes({
      "/v1/detections": () =>
        mockJsonResponse(200, {
          detections: [{ ...scenario1Detection, conditions: {} }],
          total: 1,
        }),
    });
    const error = await fetchDetections().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(DetectionsFetchError);
    expect((error as DetectionsFetchError).kind).toBe("unavailable");
  });

  it("propagates a genuine AbortError untouched, never wrapped as a DetectionsFetchError", async () => {
    const abortError = new DOMException("Aborted", "AbortError");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(abortError));
    await expect(fetchDetections()).rejects.toBe(abortError);
  });
});
