import { LoaderCircle, MessagesSquare } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Field,
  FieldError,
  FieldLabel,
} from "@/components/ui/field";
import { Input } from "@/components/ui/input";

type ConnectActionsBase = {
  disabled?: boolean;
  isConnecting: boolean;
  connectError?: string | null;
};

export type ConnectActionsProps = ConnectActionsBase &
  (
    | {
        provider: "google";
        accountEmail: string;
        onAccountEmailChange: (value: string) => void;
        onConnect: () => void;
      }
    | {
        provider: "github";
        oauthAvailable: boolean;
        authMode: string;
        pat: string;
        onPatChange: (value: string) => void;
        onOAuthConnect: () => void;
        onPatConnect: () => void;
      }
    | {
        provider: "slack";
        oauthAvailable: boolean;
        onOAuthConnect: () => void;
      }
  );

export function ConnectActions(props: ConnectActionsProps) {
  const { disabled = false, isConnecting, connectError } = props;

  if (props.provider === "google") {
    return (
      <div className="space-y-3">
        <div className="grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
          <Field>
            <FieldLabel htmlFor="google-account-email">
              Google account email
            </FieldLabel>
            <Input
              id="google-account-email"
              type="email"
              value={props.accountEmail}
              placeholder="you@example.com"
              onChange={(event) => props.onAccountEmailChange(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  void props.onConnect();
                }
              }}
            />
          </Field>
          <Button
            type="button"
            disabled={!props.accountEmail.trim() || disabled || isConnecting}
            onClick={() => void props.onConnect()}
          >
            {isConnecting ? (
              <LoaderCircle className="size-4 animate-spin" />
            ) : (
              "Connect"
            )}
          </Button>
        </div>
        {connectError ? <FieldError>{connectError}</FieldError> : null}
      </div>
    );
  }

  if (props.provider === "github") {
    return (
      <div className="space-y-3">
        {props.oauthAvailable ? (
          <Button
            type="button"
            disabled={disabled || isConnecting}
            onClick={() => void props.onOAuthConnect()}
          >
            {isConnecting ? (
              <LoaderCircle className="size-4 animate-spin" />
            ) : (
              "Connect with GitHub"
            )}
          </Button>
        ) : null}

        <details
          className="rounded-md border border-border/70 p-3"
          open={props.authMode === "local"}
        >
          <summary className="cursor-pointer text-sm font-medium">
            Connect with a personal access token
          </summary>
          <p className="mt-1 text-xs text-muted-foreground">
            Local/advanced mode. The token is validated with GitHub and stored
            only in the OS keychain.
          </p>
          <div className="mt-3 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
            <Field>
              <FieldLabel htmlFor="github-pat">Personal access token</FieldLabel>
              <Input
                id="github-pat"
                type="password"
                autoComplete="off"
                value={props.pat}
                placeholder="ghp_… or github_pat_…"
                onChange={(event) => props.onPatChange(event.target.value)}
                onKeyDown={(event) => {
                  if (event.key === "Enter") {
                    void props.onPatConnect();
                  }
                }}
              />
            </Field>
            <Button
              type="button"
              disabled={!props.pat.trim() || disabled || isConnecting}
              onClick={() => void props.onPatConnect()}
            >
              {isConnecting ? (
                <LoaderCircle className="size-4 animate-spin" />
              ) : (
                "Connect"
              )}
            </Button>
          </div>
        </details>

        {connectError ? <FieldError>{connectError}</FieldError> : null}
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {props.oauthAvailable ? (
        <Button
          type="button"
          disabled={disabled || isConnecting}
          onClick={() => void props.onOAuthConnect()}
        >
          {isConnecting ? (
            <LoaderCircle className="size-4 animate-spin" />
          ) : (
            <>
              <MessagesSquare className="size-4" />
              Connect with Slack
            </>
          )}
        </Button>
      ) : (
        <p className="text-sm text-muted-foreground">
          Slack OAuth is not configured for this build.
        </p>
      )}

      {connectError ? <FieldError>{connectError}</FieldError> : null}
    </div>
  );
}
