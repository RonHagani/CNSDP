import { describe, expect, it, vi } from "vitest";
import { mockJsonResponse, stubFetchRoutes } from "@/test/mockFetch";
import { SubmissionsFetchError, fetchSubmissions } from "./submissionsSource";

const pendingRow = {
  submissionId: 1,
  status: "admitted",
  auditId: "a1",
  auditStage: "ResponseComplete",
  createdAt: "2026-08-02T12:00:00Z",
  validationOutcome: { available: false },
};

const invalidRow = {
  submissionId: 2,
  status: "validated",
  auditId: "a2",
  auditStage: "ResponseComplete",
  createdAt: "2026-08-02T12:00:01Z",
  validationOutcome: { available: true, outcome: "invalid", reason: "malformed request" },
};

describe("fetchSubmissions", () => {
  it("requests GET /v1/submissions with no query string when called with no params", async () => {
    const fetchMock = stubFetchRoutes({
      "/v1/submissions": () => mockJsonResponse(200, { submissions: [], nextCursor: null, total: 0 }),
    });
    await fetchSubmissions();
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/submissions", expect.anything());
  });

  it("includes an outcome query param when a filter is given", async () => {
    const fetchMock = stubFetchRoutes({
      "/v1/submissions?outcome=invalid": () => mockJsonResponse(200, { submissions: [], nextCursor: null, total: 0 }),
    });
    await fetchSubmissions({ outcome: "invalid" });
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/submissions?outcome=invalid", expect.anything());
  });

  it("includes a cursor query param when a cursor is given", async () => {
    const fetchMock = stubFetchRoutes({
      "/v1/submissions?cursor=42": () => mockJsonResponse(200, { submissions: [], nextCursor: null, total: 0 }),
    });
    await fetchSubmissions({ cursor: 42 });
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/submissions?cursor=42", expect.anything());
  });

  it("combines outcome and cursor when both are given", async () => {
    const fetchMock = stubFetchRoutes({
      "/v1/submissions?outcome=pending&cursor=7": () => mockJsonResponse(200, { submissions: [], nextCursor: null, total: 0 }),
    });
    await fetchSubmissions({ outcome: "pending", cursor: 7 });
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/submissions?outcome=pending&cursor=7", expect.anything());
  });

  it("omits cursor when it is null, but keeps outcome", async () => {
    const fetchMock = stubFetchRoutes({
      "/v1/submissions?outcome=valid": () => mockJsonResponse(200, { submissions: [], nextCursor: null, total: 0 }),
    });
    await fetchSubmissions({ outcome: "valid", cursor: null });
    expect(fetchMock).toHaveBeenCalledWith("/api/v1/submissions?outcome=valid", expect.anything());
  });

  it("resolves the backend's submissions, nextCursor, and total exactly as returned", async () => {
    stubFetchRoutes({
      "/v1/submissions": () => mockJsonResponse(200, { submissions: [pendingRow, invalidRow], nextCursor: 2, total: 5 }),
    });
    const result = await fetchSubmissions();
    expect(result).toEqual({ submissions: [pendingRow, invalidRow], nextCursor: 2, total: 5 });
  });

  it("resolves a pending row with available:false and no fabricated outcome/reason", async () => {
    stubFetchRoutes({
      "/v1/submissions": () => mockJsonResponse(200, { submissions: [pendingRow], nextCursor: null, total: 1 }),
    });
    const result = await fetchSubmissions();
    expect(result.submissions[0].validationOutcome).toEqual({ available: false });
    expect(result.submissions[0].validationOutcome.outcome).toBeUndefined();
  });

  it("rejects with an unauthorized SubmissionsFetchError for a 401", async () => {
    stubFetchRoutes({ "/v1/submissions": () => mockJsonResponse(401, {}) });
    const error = await fetchSubmissions().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(SubmissionsFetchError);
    expect((error as SubmissionsFetchError).kind).toBe("unauthorized");
  });

  it("rejects with a 400 (bad request) mapped to unavailable, e.g. an invalid outcome filter", async () => {
    stubFetchRoutes({ "/v1/submissions": () => mockJsonResponse(400, {}) });
    const error = await fetchSubmissions({ outcome: "invalid" }).catch((e: unknown) => e);
    expect(error).toBeInstanceOf(SubmissionsFetchError);
    expect((error as SubmissionsFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable SubmissionsFetchError for a 5xx", async () => {
    stubFetchRoutes({ "/v1/submissions": () => mockJsonResponse(503, {}) });
    const error = await fetchSubmissions().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(SubmissionsFetchError);
    expect((error as SubmissionsFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable SubmissionsFetchError on a genuine network failure", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    const error = await fetchSubmissions().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(SubmissionsFetchError);
    expect((error as SubmissionsFetchError).kind).toBe("unavailable");
  });

  it("rejects with an unavailable SubmissionsFetchError when the body does not match the contract shape", async () => {
    stubFetchRoutes({ "/v1/submissions": () => mockJsonResponse(200, { unexpected: true }) });
    const error = await fetchSubmissions().catch((e: unknown) => e);
    expect(error).toBeInstanceOf(SubmissionsFetchError);
    expect((error as SubmissionsFetchError).kind).toBe("unavailable");
  });

  it("propagates a genuine AbortError untouched, never wrapped as a SubmissionsFetchError", async () => {
    const abortError = new DOMException("Aborted", "AbortError");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(abortError));
    await expect(fetchSubmissions()).rejects.toBe(abortError);
  });
});
