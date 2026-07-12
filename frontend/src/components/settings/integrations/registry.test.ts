// @vitest-environment jsdom

import { describe, expect, it } from "vitest";
import { integrationRegistry } from "./registry";

describe("integrationRegistry", () => {
  it("registers Google with ConfigSlot only", () => {
    const google = integrationRegistry.find((entry) => entry.id === "google");
    expect(google).toBeDefined();
    expect(google?.ConfigSlot).toBeDefined();
    expect(google?.Panel).toBeUndefined();
    expect(google?.kind).toBe("calendar_source");
  });

  it("registers GitHub with ConfigSlot only", () => {
    const github = integrationRegistry.find((entry) => entry.id === "github");
    expect(github).toBeDefined();
    expect(github?.ConfigSlot).toBeDefined();
    expect(github?.Panel).toBeUndefined();
    expect(github?.kind).toBe("activity_evidence");
  });

  it("registers Slack with Panel only", () => {
    const slack = integrationRegistry.find((entry) => entry.id === "slack");
    expect(slack).toBeDefined();
    expect(slack?.Panel).toBeDefined();
    expect(slack?.ConfigSlot).toBeUndefined();
    expect(slack?.kind).toBe("activity_evidence");
  });
});
