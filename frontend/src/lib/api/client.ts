import { API_BASE_PATH } from "./config";

/**
 * The typed fetch layer every production data source (alertSource.ts,
 * alertInventorySource.ts) calls through — the one place request
 * construction and HTTP-status-to-error-kind mapping live, so no
 * component or feature module writes raw `fetch` logic of its own.
 *
 * Deliberately carries no credential of any kind and sets no
 * credential-bearing request header. The browser calls the same-origin
 * `/api/*` prefix only; reaching the real backend is a server-side
 * concern, handled by the Vite dev/preview proxy (vite.config.ts) from a
 * non-`VITE_`-prefixed environment variable that is never bundled into
 * client code. A service credential must never be constructed or held in
 * browser-reachable code, storage, or configuration (see config.ts's own
 * doc comment).
 */
export type ApiErrorKind =
  | "unauthorized"
  | "not-found"
  | "client-error"
  | "server-error"
  | "network-error"
  | "malformed-response";

export class ApiError extends Error {
  readonly kind: ApiErrorKind;
  readonly status?: number;

  constructor(kind: ApiErrorKind, message: string, status?: number) {
    super(message);
    this.name = "ApiError";
    this.kind = kind;
    this.status = status;
  }
}

function isAbortError(err: unknown): boolean {
  return err instanceof DOMException && err.name === "AbortError";
}

/**
 * Issues a same-origin, credential-free GET against
 * `${API_BASE_PATH}${path}` and returns the parsed-but-unvalidated JSON
 * body. Reaching the real backend, if the proxy is configured for one,
 * happens entirely server-side (vite.config.ts) — this function never
 * attaches a credential-bearing header itself. Callers validate the
 * response shape themselves (types/validate.ts) — this layer only knows
 * about transport and HTTP status, never a specific response contract.
 *
 * An aborted request (component unmount, navigation away, a superseded
 * query) rethrows the native `AbortError` untouched rather than wrapping
 * it as an `ApiError` — TanStack Query recognizes that signal itself and
 * suppresses it from ever reaching `isError`/`onError`, so a stale
 * in-flight request can never overwrite newer state.
 */
export async function apiGet(path: string, options: { signal?: AbortSignal } = {}): Promise<unknown> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE_PATH}${path}`, {
      method: "GET",
      headers: {
        Accept: "application/json",
      },
      signal: options.signal,
    });
  } catch (err) {
    if (isAbortError(err)) throw err;
    throw new ApiError(
      "network-error",
      "The request could not be sent — the backend may be unreachable or a network error occurred.",
    );
  }

  if (response.status === 401 || response.status === 403) {
    throw new ApiError("unauthorized", "The request was rejected: missing or invalid credentials.", response.status);
  }
  if (response.status === 404) {
    throw new ApiError("not-found", "The requested resource does not exist.", response.status);
  }
  if (response.status >= 500) {
    throw new ApiError("server-error", "The backend reported an internal error.", response.status);
  }
  if (!response.ok) {
    throw new ApiError("client-error", `The request was rejected (status ${response.status}).`, response.status);
  }

  try {
    return await response.json();
  } catch (err) {
    if (isAbortError(err)) throw err;
    throw new ApiError("malformed-response", "The response body was not valid JSON.");
  }
}
