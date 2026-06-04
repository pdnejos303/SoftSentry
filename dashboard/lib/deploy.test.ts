import { describe, expect, it } from "vitest";
import {
  deployStatus,
  installerUrl,
  isNeverExpiry,
  type DeploymentToken,
} from "./deploy";

const base: DeploymentToken = {
  uuid: "u1",
  label: null,
  expires_at: "2999-01-01T00:00:00Z",
  max_uses: null,
  use_count: 0,
  revoked_at: null,
  created_at: "2026-06-04T00:00:00Z",
};

const now = new Date("2026-06-04T12:00:00Z");

describe("isNeverExpiry", () => {
  it("treats the year-2999 sentinel as never", () => {
    expect(isNeverExpiry("2999-01-01T00:00:00Z")).toBe(true);
  });
  it("treats a real date as expiring", () => {
    expect(isNeverExpiry("2026-12-01T00:00:00Z")).toBe(false);
  });
});

describe("deployStatus", () => {
  it("is active for an unused, unlimited, non-expiring token", () => {
    expect(deployStatus(base, now)).toBe("active");
  });
  it("is revoked when revoked_at set", () => {
    expect(deployStatus({ ...base, revoked_at: "2026-06-04T01:00:00Z" }, now)).toBe("revoked");
  });
  it("is expired past a real expiry", () => {
    expect(deployStatus({ ...base, expires_at: "2026-06-04T06:00:00Z" }, now)).toBe("expired");
  });
  it("is exhausted when uses reach max", () => {
    expect(deployStatus({ ...base, max_uses: 2, use_count: 2 }, now)).toBe("exhausted");
  });
  it("prefers revoked over exhausted", () => {
    const t = { ...base, revoked_at: "x", max_uses: 1, use_count: 5 };
    expect(deployStatus(t, now)).toBe("revoked");
  });
});

describe("installerUrl", () => {
  it("builds a link carrying token + server + os/arch", () => {
    const url = installerUrl("tok", "http://192.168.1.50:8001/");
    expect(url).toContain("/deploy/installer?");
    expect(url).toContain("token=tok");
    expect(url).toContain("os=windows");
    expect(url).toContain("server=http%3A%2F%2F192.168.1.50%3A8001");
  });

  it("downloads from the reachable server host, not localhost", () => {
    // A link opened on another machine must hit the LAN address, not its own box.
    const url = installerUrl("tok", "http://192.168.1.50:8001");
    expect(url.startsWith("http://192.168.1.50:8001/api/v1/deploy/installer")).toBe(true);
  });
});
