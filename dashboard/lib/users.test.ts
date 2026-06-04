import { describe, expect, it } from "vitest";
import { deviceLabel, roleVariant, statusVariant } from "./users";

describe("roleVariant", () => {
  it("admin is default, viewer is muted", () => {
    expect(roleVariant("admin")).toBe("default");
    expect(roleVariant("viewer")).toBe("muted");
  });
});

describe("statusVariant", () => {
  it("active is success, inactive is muted", () => {
    expect(statusVariant(true)).toBe("success");
    expect(statusVariant(false)).toBe("muted");
  });
});

describe("deviceLabel", () => {
  it("returns dash for null", () => {
    expect(deviceLabel(null)).toBe("—");
  });

  it("parses Chrome on Windows", () => {
    const ua =
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0 Safari/537.36";
    expect(deviceLabel(ua)).toBe("Chrome · Windows");
  });

  it("detects Edge over Chrome token", () => {
    const ua =
      "Mozilla/5.0 (Windows NT 10.0) Chrome/120.0 Safari/537.36 Edg/120.0";
    expect(deviceLabel(ua)).toBe("Edge · Windows");
  });

  it("labels API clients", () => {
    expect(deviceLabel("python-httpx/0.28.1")).toBe("API client · Unknown OS");
  });

  it("detects macOS Safari", () => {
    const ua =
      "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 Safari/605.1.15";
    expect(deviceLabel(ua)).toBe("Safari · macOS");
  });
});
