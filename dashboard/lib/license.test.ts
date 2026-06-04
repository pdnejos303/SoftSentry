import { describe, expect, it } from "vitest";
import {
  LICENSE_STATUSES,
  complianceSlices,
  formatCost,
  licenseStatusVariant,
  statusColor,
} from "./license";
import type { ComplianceSummary } from "./types";

describe("LICENSE_STATUSES", () => {
  it("lists the four compliance statuses", () => {
    expect(LICENSE_STATUSES).toEqual([
      "compliant",
      "over_used",
      "expiring_soon",
      "expired",
    ]);
  });
});

describe("licenseStatusVariant", () => {
  it("maps each status to a badge variant", () => {
    expect(licenseStatusVariant("compliant")).toBe("success");
    expect(licenseStatusVariant("over_used")).toBe("danger");
    expect(licenseStatusVariant("expired")).toBe("danger");
    expect(licenseStatusVariant("expiring_soon")).toBe("warning");
  });
  it("falls back to muted for unknown", () => {
    expect(licenseStatusVariant("bogus")).toBe("muted");
  });
});

describe("statusColor", () => {
  it("gives a distinct color per known status", () => {
    const colors = LICENSE_STATUSES.map(statusColor);
    expect(new Set(colors).size).toBe(colors.length);
  });
});

function summary(over: Partial<ComplianceSummary> = {}): ComplianceSummary {
  return {
    total: 0,
    compliant: 0,
    over_used: 0,
    expired: 0,
    expiring_30: 0,
    expiring_60: 0,
    expiring_90: 0,
    compliance_rate: 0,
    ...over,
  };
}

describe("complianceSlices", () => {
  it("folds the expiring buckets into one slice and drops empties", () => {
    const slices = complianceSlices(
      summary({ total: 10, compliant: 6, over_used: 2, expiring_30: 1, expiring_90: 1 }),
    );
    expect(slices.map((s) => s.key)).toEqual(["compliant", "over_used", "expiring_soon"]);
    const expiring = slices.find((s) => s.key === "expiring_soon");
    expect(expiring?.count).toBe(2);
  });

  it("returns nothing when there are no licenses", () => {
    expect(complianceSlices(summary())).toEqual([]);
  });
});

describe("formatCost", () => {
  it("renders a currency symbol with grouped thousands when known", () => {
    expect(formatCost(250000, "THB")).toBe("฿250,000");
    expect(formatCost(1500.5, "USD")).toBe("$1,500.5");
  });
  it("falls back to the currency code when no symbol is known", () => {
    expect(formatCost(1000, "SEK")).toBe("1,000 SEK");
  });
  it("renders a dash for no cost", () => {
    expect(formatCost(null, "THB")).toBe("—");
  });
});
