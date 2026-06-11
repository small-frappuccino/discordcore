import {
  type AuthSessionResponse,
  type ControlAuthProbe,
  type DiscordOAuthStatusResponse,
} from "./domains/auth";

export interface ControlApiClientConfig {
  baseUrl: string;
}

const transientGetRetryStatuses = new Set([502, 504]);
const transientGetRetryDelaysMs = [80, 160];

function normalizeBaseUrl(raw: string): string {
  return raw.trim().replace(/\/+$/, "");
}

function normalizeDashboardNextPath(raw: string): string {
  const trimmed = raw.trim();
  if (trimmed === "/") {
    return "/";
  }
  if (trimmed === "" || trimmed === "/manage") {
    return "/manage/";
  }
  if (!trimmed.startsWith("/manage/")) {
    return "/manage/";
  }
  return trimmed;
}

function requiresCSRFHeader(
  method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE",
): boolean {
  return (
    method === "POST" ||
    method === "PUT" ||
    method === "PATCH" ||
    method === "DELETE"
  );
}

export function delay(ms: number) {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}

export class ControlApiClient {
  private readonly baseUrl: string;
  private csrfToken = "";
  private csrfLoadPromise: Promise<string> | null = null;

  constructor(config: ControlApiClientConfig) {
    this.baseUrl = normalizeBaseUrl(config.baseUrl);
  }

  getBaseUrl(): string {
    return this.baseUrl;
  }

  async getSessionStatus(): Promise<ControlAuthProbe> {
    const url = this.baseUrl === "" ? "/auth/me" : `${this.baseUrl}/auth/me`;
    const response = await this.fetchWithTransientGetRetry(url);

    if (response.status === 401) {
      return { status: "unauthorized" };
    }
    if (response.status === 503) {
      return { status: "oauth_unavailable" };
    }
    if (!response.ok) {
      const text = await response.text();
      throw new Error(
        `Control API GET /auth/me failed: ${response.status} ${response.statusText} - ${text}`.trim(),
      );
    }

    const payload = (await response.json()) as AuthSessionResponse;
    const csrfToken = payload.csrf_token?.trim() ?? "";
    if (csrfToken === "") {
      throw new Error("Control API /auth/me response missing csrf_token");
    }

    this.csrfToken = csrfToken;
    return { status: "authenticated", session: payload };
  }

  async logout(): Promise<void> {
    await this.request<Record<string, unknown>>("POST", "/auth/logout");
    this.clearCSRFToken();
  }

  async getDiscordOAuthStatus(
    nextPath = "/manage/",
  ): Promise<DiscordOAuthStatusResponse> {
    const next = normalizeDashboardNextPath(nextPath);
    const suffix = next === "" ? "" : `?next=${encodeURIComponent(next)}`;
    const url =
      this.baseUrl === ""
        ? `/auth/discord/status${suffix}`
        : `${this.baseUrl}/auth/discord/status${suffix}`;
    const response = await this.fetchWithTransientGetRetry(url);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(
        `Control API GET /auth/discord/status failed: ${response.status} ${response.statusText} - ${text}`.trim(),
      );
    }
    return (await response.json()) as DiscordOAuthStatusResponse;
  }

  async request<T>(
    method: "GET" | "POST" | "PUT" | "PATCH" | "DELETE",
    path: string,
    body?: unknown,
    retryOnCSRF = true,
  ): Promise<T> {
    const url = this.baseUrl === "" ? path : `${this.baseUrl}${path}`;

    const headers = new Headers();
    if (body !== undefined) {
      headers.set("Content-Type", "application/json");
    }
    if (requiresCSRFHeader(method)) {
      const csrfToken = await this.getCSRFToken();
      headers.set("X-CSRF-Token", csrfToken);
    }

    let response: Response;
    let retries = 0;
    const maxRetries = 5;
    const occRetryDelaysMs = [250, 500, 1000, 2000, 4000];

    while (true) {
      response = await fetch(url, {
        method,
        headers,
        credentials: "include",
        body: body !== undefined ? JSON.stringify(body) : undefined,
      });

      if (
        (response.status === 412 || response.status === 428) &&
        method !== "GET"
      ) {
        if (retries < maxRetries) {
          const delay = occRetryDelaysMs[retries] * (1 + Math.random() * 0.2);
          await new Promise((resolve) => setTimeout(resolve, delay));
          retries++;
          continue;
        }
      }
      break;
    }
    const resolvedResponse =
      method === "GET" && transientGetRetryStatuses.has(response.status)
        ? await this.retryTransientGetRequest(
            url,
            {
              method,
              headers,
              credentials: "include",
              body: body !== undefined ? JSON.stringify(body) : undefined,
            },
            response,
          )
        : response;

    if (
      resolvedResponse.status === 403 &&
      retryOnCSRF &&
      requiresCSRFHeader(method)
    ) {
      this.clearCSRFToken();
      return this.request<T>(method, path, body, false);
    }

    if (!resolvedResponse.ok) {
      const text = await resolvedResponse.text();
      throw new Error(
        `Control API ${method} ${path} failed: ${resolvedResponse.status} ${resolvedResponse.statusText} - ${text}`.trim(),
      );
    }

    if (resolvedResponse.status === 204) {
      return {} as T;
    }
    return (await resolvedResponse.json()) as T;
  }

  private async fetchWithTransientGetRetry(url: string) {
    const response = await fetch(url, {
      method: "GET",
      credentials: "include",
    });
    return this.retryTransientGetRequest(
      url,
      {
        method: "GET",
        credentials: "include",
      },
      response,
    );
  }

  private async retryTransientGetRequest(
    url: string,
    init: RequestInit,
    initialResponse: Response,
  ) {
    let response = initialResponse;

    for (const delayMs of transientGetRetryDelaysMs) {
      if (!transientGetRetryStatuses.has(response.status)) {
        return response;
      }
      await delay(delayMs);
      response = await fetch(url, init);
    }

    return response;
  }

  private async getCSRFToken(): Promise<string> {
    if (this.csrfToken !== "") {
      return this.csrfToken;
    }
    if (this.csrfLoadPromise !== null) {
      return this.csrfLoadPromise;
    }

    this.csrfLoadPromise = (async () => {
      const probe = await this.getSessionStatus();
      if (probe.status !== "authenticated") {
        if (probe.status === "oauth_unavailable") {
          throw new Error(
            "Discord OAuth is not configured on this control server.",
          );
        }
        throw new Error(
          "Unauthorized. Sign in with Discord before changing dashboard settings.",
        );
      }
      return probe.session.csrf_token.trim();
    })();

    try {
      return await this.csrfLoadPromise;
    } finally {
      this.csrfLoadPromise = null;
    }
  }

  private clearCSRFToken() {
    this.csrfToken = "";
    this.csrfLoadPromise = null;
  }
}
