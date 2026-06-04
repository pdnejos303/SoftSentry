import { describe, expect, it } from "vitest";
import { LOCALES, isLocale, localeLabel } from "./locale";

describe("locale helpers", () => {
  it("exposes exactly the supported locales", () => {
    expect(LOCALES).toEqual(["th", "en", "ja"]);
  });

  it("labels locales with their native endonym", () => {
    expect(localeLabel("th")).toBe("ไทย");
    expect(localeLabel("en")).toBe("English");
    expect(localeLabel("ja")).toBe("日本語");
  });

  it("falls back to an upper-cased code for unknown locales", () => {
    expect(localeLabel("de")).toBe("DE");
  });

  it("narrows known locale codes", () => {
    expect(isLocale("th")).toBe(true);
    expect(isLocale("en")).toBe(true);
    expect(isLocale("ja")).toBe(true);
    expect(isLocale("fr")).toBe(false);
  });
});
