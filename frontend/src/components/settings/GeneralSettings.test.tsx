// @vitest-environment jsdom

import { cleanup, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { GeneralSettings } from "./GeneralSettings";

vi.mock("./useJsonSetting", () => ({
  useJsonSetting: (_key: string, fallback: unknown) => ({
    value: fallback,
    setValue: vi.fn(),
    isLoading: false,
    isSaving: false,
    error: null,
  }),
}));

vi.mock("@/lib/api", () => ({
  useLogPath: () => ({ data: "/tmp/shiet.log" }),
  useRevealLogFolder: () => ({
    isPending: false,
    mutate: vi.fn(),
  }),
}));

vi.mock("@/lib/api/shietService", () => ({
  isShietAppAvailable: () => false,
}));

vi.mock("../../../wailsjs/runtime/runtime", () => ({
  Environment: async () => ({ platform: "darwin" }),
}));

describe("GeneralSettings", () => {
  beforeEach(() => {
    cleanup();
  });

  it("keeps period cadence but drops flat target hours and workday start", () => {
    render(<GeneralSettings />);

    expect(screen.getByLabelText("Cadence")).toBeTruthy();
    expect(screen.queryByLabelText("Target hours")).toBeNull();
    expect(screen.queryByLabelText("Workday starts")).toBeNull();
  });
});
