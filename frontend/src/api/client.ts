type ErrorData = { desc?: unknown };

export class ApiClientError extends Error {
  readonly status: number;
  readonly code: number;

  constructor(status: number, code: number, message: string) {
    super(message);
    this.name = "ApiClientError";
    this.status = status;
    this.code = code;
  }
}

export type ApiQuery = Record<
  string,
  string | number | boolean | null | undefined
>;

function queryString(query?: ApiQuery): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query ?? {})) {
    if (value !== undefined && value !== null && value !== "")
      params.set(key, String(value));
  }
  return params.toString();
}

export async function apiRequest<T>(
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE",
  path: string,
  options: { query?: ApiQuery; body?: unknown } = {},
): Promise<T> {
  const url = new URL(path, window.location.origin);
  const search = queryString(options.query);
  if (search) url.search = search;

  let response: Response;
  try {
    response = await fetch(url, {
      method,
      credentials: "same-origin",
      headers: {
        accept: "application/json",
        ...(options.body === undefined
          ? {}
          : { "content-type": "application/json" }),
      },
      ...(options.body === undefined
        ? {}
        : { body: JSON.stringify(options.body) }),
    });
  } catch (error) {
    throw new ApiClientError(
      0,
      1,
      error instanceof Error ? error.message : "无法连接服务",
    );
  }

  const contentType = response.headers.get("content-type") ?? "";
  let body: unknown;
  if (contentType.includes("application/json")) {
    try {
      body = await response.json();
    } catch {
      body = undefined;
    }
  } else {
    const text = await response.text();
    body = text || undefined;
  }
  const envelope =
    body && typeof body === "object"
      ? (body as { code?: unknown; data?: unknown })
      : undefined;
  const code =
    typeof envelope?.code === "number" ? envelope.code : response.ok ? 0 : 1;
  if (!response.ok || code !== 0) {
    const data =
      envelope?.data && typeof envelope.data === "object"
        ? (envelope.data as ErrorData)
        : undefined;
    const message =
      typeof data?.desc === "string"
        ? data.desc
        : typeof body === "string"
          ? body.trim()
          : code === 401
            ? "登录状态已失效"
            : `请求失败（HTTP ${response.status}）`;
    throw new ApiClientError(
      code === 401 ? 401 : response.status,
      code,
      message,
    );
  }
  return body as T;
}

export function apiGet<T>(path: string, query?: ApiQuery) {
  return apiRequest<T>("GET", path, { query });
}
export function apiPost<T>(path: string, body?: unknown) {
  return apiRequest<T>("POST", path, { body });
}
export function apiPut<T>(path: string, body?: unknown) {
  return apiRequest<T>("PUT", path, { body });
}
export function apiPatch<T>(path: string, body?: unknown) {
  return apiRequest<T>("PATCH", path, { body });
}
export function apiDelete<T>(path: string, query?: ApiQuery) {
  return apiRequest<T>("DELETE", path, { query });
}

export function errorMessage(error: unknown): string {
  if (error instanceof ApiClientError && error.status === 0)
    return "无法连接服务，请检查网络后重试";
  return error instanceof Error && error.message
    ? error.message
    : "操作失败，请稍后重试";
}
