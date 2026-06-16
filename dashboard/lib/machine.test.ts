import { describe, expect, it } from "vitest";
import { machineLabel } from "./machine";

describe("machineLabel", () => {
  it("prefers display_name when set", () => {
    expect(machineLabel({ display_name: "CEO Laptop", hostname: "DESKTOP-AB12" })).toBe(
      "CEO Laptop",
    );
  });

  it("falls back to hostname when display_name is null", () => {
    expect(machineLabel({ display_name: null, hostname: "DESKTOP-AB12" })).toBe("DESKTOP-AB12");
  });

  it("falls back to hostname when display_name is blank/whitespace", () => {
    expect(machineLabel({ display_name: "   ", hostname: "DESKTOP-AB12" })).toBe("DESKTOP-AB12");
  });
});
