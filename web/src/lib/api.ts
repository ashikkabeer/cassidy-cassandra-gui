// Typed fetch wrapper for the Cassidy backend.
//
// - Always sends cookies (`credentials: 'include'`).
// - For mutating verbs, reads the `cassidy_csrf` cookie and forwards it as the
//   `X-CSRF-Token` header (matching the double-submit middleware on the server).
// - Throws `APIError` with the server's `{ code, message }` for callers to
//   match on (e.g. show "invalid credentials" vs "rate limited").

export class APIError extends Error {
  status: number;
  code: string;
  firstRun?: boolean;
  constructor(status: number, code: string, message: string, firstRun?: boolean) {
    super(message);
    this.status = status;
    this.code = code;
    this.firstRun = firstRun;
  }
}

const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);

function readCookie(name: string): string | null {
  const target = `${name}=`;
  for (const part of document.cookie.split(";")) {
    const c = part.trim();
    if (c.startsWith(target)) return decodeURIComponent(c.slice(target.length));
  }
  return null;
}

interface RequestOpts {
  method?: string;
  body?: unknown;
  signal?: AbortSignal;
}

export async function api<T = unknown>(path: string, opts: RequestOpts = {}): Promise<T> {
  const method = (opts.method ?? "GET").toUpperCase();
  const headers: Record<string, string> = {};
  if (opts.body !== undefined) headers["Content-Type"] = "application/json";
  if (!SAFE_METHODS.has(method)) {
    const csrf = readCookie("cassidy_csrf");
    if (csrf) headers["X-CSRF-Token"] = csrf;
  }
  const res = await fetch(`/api/v1${path}`, {
    method,
    credentials: "include",
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
    signal: opts.signal,
  });
  if (res.status === 204) return undefined as T;

  const text = await res.text();
  let parsed: unknown = null;
  try {
    parsed = text ? JSON.parse(text) : null;
  } catch {
    /* fall through */
  }
  if (!res.ok) {
    const err = (parsed as { error?: { code?: string; message?: string }; first_run?: boolean })?.error;
    throw new APIError(
      res.status,
      err?.code ?? "unknown",
      err?.message ?? `HTTP ${res.status}`,
      (parsed as { first_run?: boolean })?.first_run,
    );
  }
  return parsed as T;
}
