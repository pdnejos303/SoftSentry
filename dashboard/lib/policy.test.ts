import { describe, expect, it } from "vitest";
import { alertStatusVariant, severityVariant } from "./policy";

describe("severityVariant", () => {
  it("maps high to danger", () => {
    expect(severityVariant("high")).toBe("danger");
  });
  it("maps medium to warning", () => {
    expect(severityVariant("medium")).toBe("warning");
  });
  it("maps low to muted", () => {
    expect(severityVariant("low")).toBe("muted");
  });
  it("falls back to muted for unknown", () => {
    expect(severityVariant("whatever")).toBe("muted");
  });
});

describe("alertStatusVariant", () => {
  it("maps active to danger", () => {
    expect(alertStatusVariant("active")).toBe("danger");
  });
  it("maps acknowledged to warning", () => {
    expect(alertStatusVariant("acknowledged")).toBe("warning");
  });
  it("maps resolved to success", () => {
    expect(alertStatusVariant("resolved")).toBe("success");
  });
  it("falls back to muted for unknown", () => {
    expect(alertStatusVariant("weird")).toBe("muted");
  });
});
