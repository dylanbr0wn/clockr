import { createConnectTransport } from "@connectrpc/connect-web";

let transport: ReturnType<typeof createConnectTransport> | undefined;

export function rpcTransport() {
  return (transport ??= createConnectTransport({ baseUrl: rpcBaseUrl() }));
}

function rpcBaseUrl() {
  const configured = import.meta.env.VITE_SHIET_RPC_BASE_URL?.trim();
  if (configured) return configured.replace(/\/$/, "");
  const current = new URL(window.location.href);
  return `${current.protocol}//${current.host}/rpc`;
}
