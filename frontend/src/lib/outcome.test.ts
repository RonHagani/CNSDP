import { describe, expect, it } from "vitest";
import { formatOutcome } from "./outcome";

describe("formatOutcome", () => {
  it("prefers the Kubernetes HTTP response code when present — unchanged existing behavior", () => {
    expect(formatOutcome({ code: 201 })).toBe("201");
    expect(formatOutcome({ code: 101 })).toBe("101");
  });

  it("prefers code even when successful/errorCode are also present", () => {
    expect(formatOutcome({ code: 200, successful: true })).toBe("200");
  });

  it("renders a CloudTrail error outcome (errorCode present, code absent)", () => {
    expect(formatOutcome({ errorCode: "AccessDenied", successful: false })).toBe("error: AccessDenied");
  });

  it("renders a CloudTrail success outcome (successful true, no code or errorCode)", () => {
    expect(formatOutcome({ successful: true })).toBe("success");
  });

  it("renders the Kubernetes no-outcome-recorded state as a dash, never as an error", () => {
    // Scenario 1's own real shape: no responseStatus was recorded, so
    // successful is false and errorCode is empty -- this exact combination
    // can never occur for a real CloudTrail event (Successful is defined as
    // "no recorded errorCode" on the backend), so it unambiguously means
    // "no outcome recorded" here.
    expect(formatOutcome({ successful: false })).toBe("—");
    expect(formatOutcome({})).toBe("—");
  });
});
