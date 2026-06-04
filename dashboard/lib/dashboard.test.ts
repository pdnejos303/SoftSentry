import { describe, expect, it } from "vitest";
import { breakdownRows, formatRisk, riskColor, riskFill } from "./dashboard";

describe("riskColor", () => {
  it("maps scores to threshold bands", () => {
    expect(riskColor(0)).toBe("green");
    expect(riskColor(1)).toBe("yellow");
    expect(riskColor(10)).toBe("yellow");
    expect(riskColor(11)).toBe("orange");
    expect(riskColor(30)).toBe("orange");
    expect(riskColor(31)).toBe("red");
    expect(riskColor(120)).toBe("red");
  });
});

describe("riskFill", () => {
  it("returns a distinct color per band and falls back to yellow", () => {
    const colors = ["green", "yellow", "orange", "red"].map(riskFill);
    expect(new Set(colors).size).toBe(4);
    expect(riskFill("bogus")).toBe(riskFill("yellow"));
  });
});

describe("formatRisk", () => {
  it("drops trailing .0 but keeps .5", () => {
    expect(formatRisk(6)).toBe("6");
    expect(formatRisk(0)).toBe("0");
    expect(formatRisk(6.5)).toBe("6.5");
  });
});

describe("breakdownRows", () => {
  const rows = breakdownRows({
    unsigned: 2,
    unauthorized: 1,
    cve_critical: 1,
    cve_high: 2,
    cve_medium: 3,
    cve_low: 4,
  });

  it("computes count x weight = points per signal", () => {
    const crit = rows.find((r) => r.key === "cve_critical");
    expect(crit).toMatchObject({ count: 1, weight: 5, points: 5 });
    const low = rows.find((r) => r.key === "cve_low");
    expect(low).toMatchObject({ count: 4, weight: 0.5, points: 2 });
  });

  it("totals to the same weighted sum as the backend formula", () => {
    const total = rows.reduce((sum, r) => sum + r.points, 0);
    // 2*1 + 1*3 + 1*5 + 2*3 + 3*1 + 4*0.5 = 21
    expect(total).toBe(21);
  });
});
