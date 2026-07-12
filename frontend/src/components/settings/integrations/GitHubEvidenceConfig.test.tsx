// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { GitHubEvidenceConfig } from "./GitHubEvidenceConfig";

const mocks = vi.hoisted(() => ({
  setRepoSelectedMutate: vi.fn(),
  connections: [] as Array<{ provider: string }>,
  repos: [] as Array<{
    id: number;
    fullName: string;
    private: boolean;
    selected: boolean;
  }>,
  reposLoading: false,
}));

vi.mock("@/lib/api", () => ({
  useIntegrationConnections: () => ({
    data: mocks.connections,
    isLoading: false,
  }),
  useGitHubRepos: () => ({
    data: mocks.repos,
    isLoading: mocks.reposLoading,
  }),
  useSetGitHubRepoSelected: () => ({
    isPending: false,
    mutateAsync: mocks.setRepoSelectedMutate,
  }),
}));

function createWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false },
    },
  });

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  }

  return Wrapper;
}

describe("GitHubEvidenceConfig", () => {
  beforeEach(() => {
    cleanup();
    mocks.connections = [];
    mocks.repos = [];
    mocks.reposLoading = false;
    mocks.setRepoSelectedMutate.mockReset();
  });

  it("shows connect prompt when no GitHub account is connected", () => {
    render(<GitHubEvidenceConfig />, { wrapper: createWrapper() });

    expect(
      screen.getByText("Connect a GitHub account to see repositories here."),
    ).toBeTruthy();
  });

  it("shows refresh hint when connected but no repos", () => {
    mocks.connections = [{ provider: "github" }];

    render(<GitHubEvidenceConfig />, { wrapper: createWrapper() });

    expect(
      screen.getByText(
        "No repositories found. Try Refresh repos on the connection above.",
      ),
    ).toBeTruthy();
  });

  it("toggles repo track selection", () => {
    mocks.connections = [{ provider: "github" }];
    mocks.repos = [
      {
        id: 99,
        fullName: "acme/widget",
        private: false,
        selected: false,
      },
    ];

    render(<GitHubEvidenceConfig />, { wrapper: createWrapper() });

    fireEvent.click(
      screen.getByRole("button", { name: "Track acme/widget as evidence" }),
    );

    expect(mocks.setRepoSelectedMutate).toHaveBeenCalledWith({
      repoID: 99,
      selected: true,
    });
  });
});
