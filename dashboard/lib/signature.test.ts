import { describe, expect, it } from "vitest";
import { SIGNATURE_STATUSES, signatureColor, withPercentages } from "./signature";

describe("SIGNATURE_STATUSES", () => {
  it("covers the four signed/unsigned states in display order", () => {
    expect(SIGNATURE_STATUSES).toEqual(["valid", "expired", "invalid", "unsigned"]);
  });
});

describe("signatureColor", () => {
  it("gives a distinct color per status", () => {
    const colors = SIGNATURE_STATUSES.map(signatureColor);
    expect(new Set(colors).size).toBe(colors.length);
  });
  it("falls back to a muted color for unknown", () => {
    expect(signatureColor("unknown")).toBe(signatureColor("unknown"));
    expect(typeof signatureColor("unknown")).toBe("string");
  });
});

describe("withPercentages", () => {
  it("computes integer-rounded percentages of the total", () => {
    const out = withPercentages(
      [
        { status: "valid", count: 3 },
        { status: "unsigned", count: 1 },
      ],
      4,
    );
    expect(out).toEqual([
      { status: "valid", count: 3, percentage: 75 },
      { status: "unsigned", count: 1, percentage: 25 },
    ]);
  });
  it("returns 0% when total is zero (no divide-by-zero)", () => {
    const out = withPercentages([{ status: "valid", count: 0 }], 0);
    expect(out[0]?.percentage).toBe(0);
  });
});
