// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, cleanup } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { IntegrationDetail } from "./IntegrationDetail";

vi.mock("@tanstack/react-router", () => ({
  Link: ({
    children,
    to,
  }: {
    children: ReactNode;
    to: string;
  }) => <a href={to}>{children}</a>,
}));

vi.mock("@/lib/api", () => ({
  useIntegrationConnections: () => ({
    data: [],
    isLoading: false,
  }),
  useConnectIntegration: () => ({
    isPending: false,
    mutateAsync: vi.fn(),
  }),
  useDisconnectIntegration: () => ({
    isPending: false,
    mutateAsync: vi.fn(),
  }),
  useIntegrationAuthStatus: () => ({
    data: { mode: "broker", brokerBaseUrl: "https://auth.example.com", oauthAvailable: true },
    isLoading: false,
  }),
  useRefreshGitHubRepos: () => ({
    isPending: false,
    mutateAsync: vi.fn(),
  }),
  useCalendars: () => ({
    data: [],
    isLoading: false,
  }),
  useCategories: () => ({
    data: [],
    isLoading: false,
  }),
  useSetCalendarSelected: () => ({
    isPending: false,
    mutateAsync: vi.fn(),
  }),
  useSetCalendarDefaultCategory: () => ({
    isPending: false,
    mutateAsync: vi.fn(),
  }),
  useGitHubRepos: () => ({
    data: [],
    isLoading: false,
  }),
  useSetGitHubRepoSelected: () => ({
    isPending: false,
    mutateAsync: vi.fn(),
  }),
  useRefreshSlackChannels: () => ({
    isPending: false,
    mutateAsync: vi.fn(),
  }),
  useRefreshBitbucketResources: () => ({
    isPending: false,
    mutateAsync: vi.fn(),
  }),
  useSlackChannels: () => ({
    data: [],
    isLoading: false,
  }),
  useSetSlackChannelSelected: () => ({
    isPending: false,
    mutateAsync: vi.fn(),
  }),
  useBitbucketWorkspaces: () => ({
    data: [],
    isLoading: false,
  }),
  useBitbucketRepos: () => ({
    data: [],
    isLoading: false,
  }),
  useSetBitbucketWorkspaceSelected: () => ({
    isPending: false,
    mutateAsync: vi.fn(),
  }),
  useSetBitbucketRepoSelected: () => ({
    isPending: false,
    mutateAsync: vi.fn(),
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

afterEach(() => {
  cleanup();
});

describe("IntegrationDetail", () => {
  it("renders Google connect shell and calendar config slot", () => {
    render(<IntegrationDetail providerId="google" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("Google Calendar")).toBeTruthy();
    expect(screen.getByText("Connected Google Accounts")).toBeTruthy();
    expect(screen.getByText("Calendars")).toBeTruthy();
    expect(
      screen.getByText("Connect a Google account to see calendars here."),
    ).toBeTruthy();
  });

  it("renders GitHub connect shell and repository config slot", () => {
    render(<IntegrationDetail providerId="github" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("GitHub")).toBeTruthy();
    expect(screen.getByText("Connect with GitHub")).toBeTruthy();
    expect(screen.getByText("Connected Accounts")).toBeTruthy();
    expect(screen.getByText("Repositories")).toBeTruthy();
    expect(
      screen.getByText("Connect a GitHub account to see repositories here."),
    ).toBeTruthy();
  });

  it("renders Slack connect shell and channel config slot", () => {
    render(<IntegrationDetail providerId="slack" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("Slack")).toBeTruthy();
    expect(screen.getByText("Connect with Slack")).toBeTruthy();
    expect(screen.getByText("Connected Workspaces")).toBeTruthy();
    expect(screen.getByText("Channels")).toBeTruthy();
    expect(
      screen.getByText("Connect a Slack workspace to see channels here."),
    ).toBeTruthy();
  });

  it("renders Bitbucket connect shell and workspace/repo config slots", () => {
    render(<IntegrationDetail providerId="bitbucket" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("Bitbucket")).toBeTruthy();
    expect(screen.getByText("Connect with Bitbucket")).toBeTruthy();
    expect(screen.getByText("Connected Accounts")).toBeTruthy();
    expect(screen.getByText("Workspaces")).toBeTruthy();
    expect(screen.getByText("Repositories")).toBeTruthy();
    expect(
      screen.getByText("Connect a Bitbucket account to see workspaces here."),
    ).toBeTruthy();
  });

  it("shows unknown provider message", () => {
    render(<IntegrationDetail providerId="unknown" />, {
      wrapper: createWrapper(),
    });

    expect(screen.getByText("Unknown integration provider.")).toBeTruthy();
    expect(
      screen.getByRole("link", { name: /Back to integrations/i }).getAttribute("href"),
    ).toBe("/settings/integrations");
  });
});
