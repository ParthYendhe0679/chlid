import { ApiError, parseApiError } from "./errors";

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080/api/v1";

export const SESSION_EXPIRED_EVENT = "cc:session-expired";

type Query = Record<string, string | number | boolean | undefined | null>;

export interface RequestOptions {
  method?: "GET" | "POST" | "PUT" | "PATCH" | "DELETE";
  body?: unknown;
  query?: Query;
  signal?: AbortSignal;
  timeoutMs?: number;
  /** Skip the refresh-and-retry-once flow on 401 (used by auth endpoints). */
  skipRefresh?: boolean;
}

function buildUrl(path: string, query?: Query) {
  const url = `${API_URL}${path}`;
  if (!query) return url;
  const qs = new URLSearchParams();
  for (const [k, v] of Object.entries(query)) if (v !== undefined && v !== null && v !== "") qs.set(k, String(v));
  const s = qs.toString();
  return s ? `${url}?${s}` : url;
}

let refreshing: Promise<boolean> | null = null;
async function refreshSession(): Promise<boolean> {
  refreshing ??= fetch(buildUrl("/auth/refresh"), { method: "POST", credentials: "include" })
    .then((r) => r.ok)
    .catch(() => false)
    .finally(() => { refreshing = null; });
  return refreshing;
}

export function notifySessionExpired() {
  if (typeof window !== "undefined") window.dispatchEvent(new Event(SESSION_EXPIRED_EVENT));
}

/** Typed JSON request. Only idempotent GETs are ever retried (once, on network failure). */
export async function api<T>(path: string, opts: RequestOptions = {}): Promise<T> {
  const method = opts.method ?? "GET";

  const attempt = async (): Promise<T> => {
    const ctrl = new AbortController();
    const timeout = setTimeout(() => ctrl.abort("timeout"), opts.timeoutMs ?? 30_000);
    const onAbort = () => ctrl.abort("aborted");
    opts.signal?.addEventListener("abort", onAbort);
    try {
      const res = await fetch(buildUrl(path, opts.query), {
        method,
        credentials: "include",
        headers: opts.body !== undefined ? { "Content-Type": "application/json", Accept: "application/json" } : { Accept: "application/json" },
        body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
        signal: ctrl.signal,
      });
      const text = await res.text();
      let json: unknown = null;
      if (text) { try { json = JSON.parse(text); } catch { json = null; } }
      if (!res.ok) throw parseApiError(res.status, json);
      return json as T;
    } catch (err) {
      if (err instanceof ApiError) throw err;
      if (ctrl.signal.aborted) {
        throw new ApiError(0, ctrl.signal.reason === "timeout" ? "TIMEOUT" : "ABORTED", ctrl.signal.reason === "timeout" ? "The server took too long to respond." : "Request cancelled.");
      }
      throw new ApiError(0, "NETWORK_ERROR", "Could not reach the server. Check your connection and try again.");
    } finally {
      clearTimeout(timeout);
      opts.signal?.removeEventListener("abort", onAbort);
    }
  };

  const run = async (): Promise<T> => {
    try {
      return await attempt();
    } catch (err) {
      if (err instanceof ApiError && err.code === "NETWORK_ERROR" && method === "GET") return attempt();
      throw err;
    }
  };

  try {
    return await run();
  } catch (err) {
    if (err instanceof ApiError && err.isAuth && !opts.skipRefresh) {
      if (await refreshSession()) return run();
      notifySessionExpired();
    }
    throw err;
  }
}

/**
 * Multipart upload with progress. Uses XHR because fetch has no upload progress.
 * Never retried automatically: uploads and AI checks are not idempotent.
 */
export function uploadForm<T>(
  path: string,
  form: FormData,
  opts: { onProgress?: (fraction: number) => void; signal?: AbortSignal; timeoutMs?: number } = {},
): Promise<T> {
  return new Promise<T>((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open("POST", buildUrl(path));
    xhr.withCredentials = true;
    xhr.timeout = opts.timeoutMs ?? 90_000;
    xhr.upload.onprogress = (e) => { if (e.lengthComputable) opts.onProgress?.(e.loaded / e.total); };
    xhr.onload = () => {
      let json: unknown = null;
      try { json = xhr.responseText ? JSON.parse(xhr.responseText) : null; } catch { json = null; }
      if (xhr.status >= 200 && xhr.status < 300) return resolve(json as T);
      const err = parseApiError(xhr.status, json);
      if (err.isAuth) notifySessionExpired();
      reject(err);
    };
    xhr.onerror = () => reject(new ApiError(0, "NETWORK_ERROR", "Upload failed. Check your connection and try again."));
    xhr.ontimeout = () => reject(new ApiError(0, "TIMEOUT", "The upload took too long."));
    xhr.onabort = () => reject(new ApiError(0, "ABORTED", "Upload cancelled."));
    opts.signal?.addEventListener("abort", () => xhr.abort());
    xhr.send(form);
  });
}
