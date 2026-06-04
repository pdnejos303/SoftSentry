import { describe, expect, it } from "vitest";
import { buildCron, formatBytes, reportStatusVariant } from "./reports";

describe("reportStatusVariant", () => {
  it("maps each status to a badge variant", () => {
    expect(reportStatusVariant("completed")).toBe("success");
    expect(reportStatusVariant("failed")).toBe("danger");
    expect(reportStatusVariant("running")).toBe("warning");
    expect(reportStatusVariant("queued")).toBe("muted");
  });
});

describe("formatBytes", () => {
  it("returns a dash for empty sizes", () => {
    expect(formatBytes(null)).toBe("—");
    expect(formatBytes(0)).toBe("—");
  });

  it("scales bytes into human units", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(2048)).toBe("2 KB");
    expect(formatBytes(1_572_864)).toBe("1.5 MB");
  });
});

describe("buildCron", () => {
  it("builds preset cron expressions", () => {
    expect(buildCron("daily", { hour: 9, minute: 0 })).toBe("0 9 * * *");
    expect(buildCron("weekly", { hour: 9, minute: 30, dow: 1 })).toBe("30 9 * * 1");
    expect(buildCron("monthly", { hour: 8, minute: 0, dom: 15 })).toBe("0 8 15 * *");
  });
});
