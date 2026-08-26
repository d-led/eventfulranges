import { describe, it, expect } from "vitest";
import { delayFor, RECONNECT_BASE_MS, RECONNECT_MAX_MS } from "./backoff.js";

describe("backoff", () => {
  it("waits the base delay before the first retry", () => {
    expect(delayFor(0)).toBe(RECONNECT_BASE_MS);
  });

  it("doubles the delay each attempt until the cap", () => {
    expect(delayFor(0)).toBe(1000);
    expect(delayFor(1)).toBe(2000);
    expect(delayFor(2)).toBe(4000);
    expect(delayFor(3)).toBe(8000);
    expect(delayFor(4)).toBe(16000);
    expect(delayFor(5)).toBe(RECONNECT_MAX_MS);
    expect(delayFor(6)).toBe(RECONNECT_MAX_MS);
  });
});
