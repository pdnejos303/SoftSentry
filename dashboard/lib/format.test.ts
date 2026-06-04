import { describe, expect, it } from "vitest";
import { timeAgo } from "./format";

describe("timeAgo", () => {
  const now = new Date("2026-05-31T12:00:00Z");

  it("renders seconds for very recent timestamps", () => {
    expect(timeAgo("2026-05-31T11:59:30Z", now)).toBe("30s ago");
  });

  it("renders 'just now' for <5s", () => {
    expect(timeAgo("2026-05-31T11:59:58Z", now)).toBe("just now");
  });

  it("renders minutes", () => {
    expect(timeAgo("2026-05-31T11:58:00Z", now)).toBe("2m ago");
  });

  it("renders hours", () => {
    expect(timeAgo("2026-05-31T09:00:00Z", now)).toBe("3h ago");
  });

  it("renders days", () => {
    expect(timeAgo("2026-05-29T12:00:00Z", now)).toBe("2d ago");
  });

  it("clamps future timestamps to 'just now'", () => {
    expect(timeAgo("2026-05-31T12:00:30Z", now)).toBe("just now");
  });
});
