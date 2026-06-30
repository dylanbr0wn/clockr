import { describe, expect, it } from "vitest";
import { aiEndpointsMatch, normalizeAIEndpoint } from "./endpoints";

describe("normalizeAIEndpoint", () => {
  it("treats localhost and 127.0.0.1 as equivalent", () => {
    expect(
      normalizeAIEndpoint("http://localhost:1234/v1"),
    ).toBe(normalizeAIEndpoint("http://127.0.0.1:1234/v1"));
  });

  it("ignores trailing slashes", () => {
    expect(
      normalizeAIEndpoint("http://127.0.0.1:1234/v1/"),
    ).toBe(normalizeAIEndpoint("http://127.0.0.1:1234/v1"));
  });
});

describe("aiEndpointsMatch", () => {
  it("matches equivalent local endpoints", () => {
    expect(
      aiEndpointsMatch(
        "http://localhost:1234/v1",
        "http://127.0.0.1:1234/v1",
      ),
    ).toBe(true);
  });
});
