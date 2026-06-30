/**
 * Normalizes OpenAI-compatible base URLs so localhost/127.0.0.1 and trailing
 * slashes compare equal when matching a saved preference to a detected endpoint.
 */
export function normalizeAIEndpoint(url: string): string {
  try {
    const parsed = new URL(url.trim());
    const host =
      parsed.hostname === "localhost" ? "127.0.0.1" : parsed.hostname.toLowerCase();
    const port = parsed.port ? `:${parsed.port}` : "";
    const path = parsed.pathname.replace(/\/+$/, "");
    return `${parsed.protocol}//${host}${port}${path}`.toLowerCase();
  } catch {
    return url.trim().toLowerCase();
  }
}

export function aiEndpointsMatch(a: string, b: string): boolean {
  if (!a.trim() || !b.trim()) {
    return false;
  }
  return normalizeAIEndpoint(a) === normalizeAIEndpoint(b);
}
