import { useEffect, useState, type ReactNode } from "react";
import { Environment } from "../../wailsjs/runtime/runtime";
import { AppHeader } from "@/components/app-header";

const MAC_TITLEBAR_PADDING_CLASS = "pl-24";
const DEFAULT_TITLEBAR_PADDING_CLASS = "pl-3";

function getInitialPlatform() {
  return navigator.platform.toLowerCase().includes("mac") ? "darwin" : "";
}

function useTitlebarPaddingClass() {
  const [platform, setPlatform] = useState(getInitialPlatform);

  useEffect(() => {
    let isMounted = true;

    const loadEnvironment = async () => {
      try {
        const environment = await Environment();
        if (isMounted) {
          setPlatform(environment.platform);
        }
      } catch {
        // The Wails runtime is not present when rendering in plain Vite.
      }
    };

    void loadEnvironment();

    return () => {
      isMounted = false;
    };
  }, []);

  return platform === "darwin"
    ? MAC_TITLEBAR_PADDING_CLASS
    : DEFAULT_TITLEBAR_PADDING_CLASS;
}

interface AppShellProps {
  children: ReactNode;
}

export function AppShell({ children }: AppShellProps) {
  const titlebarPaddingClass = useTitlebarPaddingClass();

  return (
    <main className="app-drag-region app-window relative h-screen overflow-hidden overscroll-none bg-background text-foreground">
      <div className="mx-auto flex h-full min-h-0 w-full flex-col">
        <AppHeader titlebarPaddingClass={titlebarPaddingClass} />
        {children}
      </div>
    </main>
  );
}
