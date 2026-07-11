import { Clock, Settings } from "lucide-react";
import { Button } from "@/components/ui/button";
import { SettingsDialog } from "@/components/settings/SettingsDialog";

interface AppHeaderProps {
  titlebarPaddingClass: string;
}

export function AppHeader({ titlebarPaddingClass }: AppHeaderProps) {
  return (
    <header
      className={`shrink-0 flex items-center gap-3 py-2 pr-3 ${titlebarPaddingClass}`}
    >
      <div className="bg-primary rounded-md text-accent p-1.5">
        <Clock className="size-4" />
      </div>
      <div>
        <h1 className="text-base font-medium">shiet</h1>
      </div>
      <div className="grow" />
      <div className="app-no-drag">
        <SettingsDialog>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            aria-label="Open settings"
          >
            <Settings className="size-4" />
          </Button>
        </SettingsDialog>
      </div>
    </header>
  );
}
