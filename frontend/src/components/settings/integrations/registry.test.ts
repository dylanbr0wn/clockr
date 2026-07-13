// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import { integrationRegistry } from "./registry";

describe("integrationRegistry", () => {
  it("registers all providers with ConfigSlot only", () => {
    for (const id of ["google", "github", "slack", "bitbucket"] as const) {
      const entry = integrationRegistry.find((item) => item.id === id);
      expect(entry).toBeDefined();
      expect(entry?.ConfigSlot).toBeDefined();
    }
  });

  it("assigns correct kinds", () => {
    const google = integrationRegistry.find((entry) => entry.id === "google");
    expect(google?.kind).toBe("calendar_source");

    for (const id of ["github", "slack", "bitbucket"] as const) {
      const entry = integrationRegistry.find((item) => item.id === id);
      expect(entry?.kind).toBe("activity_evidence");
    }
  });
});
