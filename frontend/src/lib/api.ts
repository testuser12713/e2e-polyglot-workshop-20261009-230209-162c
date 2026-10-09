export interface ApiErrorBody {
  error: { code: string; message: string };
}

export class ApiError extends Error {
  readonly status: number;
  readonly code: string;

  constructor(status: number, code: string, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
  }
}

const DEFAULT_BASE_URL = "http://localhost:8080";

function resolveBaseUrl(): string {
  const configured = import.meta.env.VITE_API_BASE_URL as string | undefined;
  const raw = configured && configured.trim() ? configured : DEFAULT_BASE_URL;
  return raw.replace(/\/+$/, "");
}

export function getBaseUrl(): string {
  return resolveBaseUrl();
}

let authToken: string | null = null;

export function setAuthToken(token: string | null): void {
  authToken = token;
}

export function getAuthToken(): string | null {
  return authToken;
}

export interface ApiFetchOptions {
  method?: string;
  body?: unknown;
  headers?: Record<string, string>;
  signal?: AbortSignal;
}

function parseErrorBody(parsed: unknown, response: Response): { code: string; message: string } {
  const body = parsed as ApiErrorBody | undefined;
  const error = body?.error;
  return {
    code: typeof error?.code === "string" && error.code ? error.code : `http_${response.status}`,
    message:
      typeof error?.message === "string" && error.message
        ? error.message
        : response.statusText || "Unbekannter Fehler",
  };
}

export async function apiFetch<T = unknown>(
  path: string,
  options: ApiFetchOptions = {},
): Promise<T> {
  const { method = "GET", body, headers = {}, signal } = options;
  const requestHeaders: Record<string, string> = {
    Accept: "application/json",
    ...headers,
  };
  const init: RequestInit = { method, headers: requestHeaders, signal };
  if (body !== undefined) {
    requestHeaders["Content-Type"] = "application/json";
    init.body = JSON.stringify(body);
  }
  if (authToken) {
    requestHeaders["Authorization"] = `Bearer ${authToken}`;
  }

  const response = await fetch(`${getBaseUrl()}${path}`, init);

  if (response.status === 204) {
    return undefined as T;
  }

  const text = await response.text();
  let parsed: unknown;
  if (text) {
    try {
      parsed = JSON.parse(text);
    } catch {
      parsed = undefined;
    }
  }

  if (!response.ok) {
    const { code, message } = parseErrorBody(parsed, response);
    throw new ApiError(response.status, code, message);
  }

  return parsed as T;
}
