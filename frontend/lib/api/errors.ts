export type ApiErrorCode =
  | "VALIDATION_ERROR" | "UNAUTHENTICATED" | "FORBIDDEN" | "NOT_FOUND" | "INVALID_TRANSITION"
  | "RATE_LIMITED" | "AI_UNAVAILABLE" | "NETWORK_ERROR" | "TIMEOUT" | "ABORTED" | "UNKNOWN";

export class ApiError extends Error {
  readonly status: number;
  readonly code: ApiErrorCode;
  readonly requestId?: string;
  constructor(status: number, code: ApiErrorCode, message: string, requestId?: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.requestId = requestId;
  }
  get isAuth() { return this.code === "UNAUTHENTICATED"; }
  get isForbidden() { return this.code === "FORBIDDEN"; }
}

const KNOWN: ApiErrorCode[] = [
  "VALIDATION_ERROR", "UNAUTHENTICATED", "FORBIDDEN", "NOT_FOUND", "INVALID_TRANSITION", "RATE_LIMITED", "AI_UNAVAILABLE",
];

/** Maps any response body/status to an ApiError using the error format in section 15.1. */
export function parseApiError(status: number, body: unknown): ApiError {
  const e = (body as { error?: { code?: string; message?: string; request_id?: string } } | null)?.error;
  let code: ApiErrorCode = "UNKNOWN";
  if (e?.code && (KNOWN as string[]).includes(e.code)) code = e.code as ApiErrorCode;
  else if (status === 400) code = "VALIDATION_ERROR";
  else if (status === 401) code = "UNAUTHENTICATED";
  else if (status === 403) code = "FORBIDDEN";
  else if (status === 404) code = "NOT_FOUND";
  else if (status === 409) code = "INVALID_TRANSITION";
  else if (status === 429) code = "RATE_LIMITED";
  else if (status === 503) code = "AI_UNAVAILABLE";
  return new ApiError(status, code, e?.message ?? `Request failed (${status})`, e?.request_id);
}
