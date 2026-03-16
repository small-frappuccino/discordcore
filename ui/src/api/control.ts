export interface EmbedUpdateTargetConfig {
  type?: "webhook_message" | "channel_message" | "";
  message_id?: string;
  channel_id?: string;
  webhook_url?: string;
}

export interface PartnerEntryConfig {
  fandom?: string;
  name: string;
  link: string;
}

export interface PartnerBoardTemplateConfig {
  title?: string;
  continuation_title?: string;
  intro?: string;
  section_header_template?: string;
  section_continuation_suffix?: string;
  section_continuation_template?: string;
  line_template?: string;
  empty_state_text?: string;
  footer_template?: string;
  other_fandom_label?: string;
  color?: number;
  disable_fandom_sorting?: boolean;
  disable_partner_sorting?: boolean;
}

export interface PartnerBoardConfig {
  target?: EmbedUpdateTargetConfig;
  template?: PartnerBoardTemplateConfig;
  partners?: PartnerEntryConfig[];
}

export interface PartnerBoardResponse {
  status: string;
  guild_id: string;
  partner_board: PartnerBoardConfig;
}

export interface TargetResponse {
  status: string;
  guild_id: string;
  target: EmbedUpdateTargetConfig;
}

export interface TemplateResponse {
  status: string;
  guild_id: string;
  template: PartnerBoardTemplateConfig;
}

export interface PartnersResponse {
  status: string;
  guild_id: string;
  partners: PartnerEntryConfig[];
}

export interface PartnerResponse {
  status: string;
  guild_id: string;
  partner: PartnerEntryConfig;
}

export interface SyncResponse {
  status: string;
  guild_id: string;
  synced: boolean;
}

export interface ManageableGuild {
  id: string;
  name: string;
  icon?: string;
  owner: boolean;
  permissions: number;
}

export interface ManageableGuildsResponse {
  status: string;
  count: number;
  guilds: ManageableGuild[];
}

export interface DiscordOAuthUser {
  id: string;
  username: string;
  discriminator?: string;
  global_name?: string;
  avatar?: string;
}

export interface AuthSessionResponse {
  status: string;
  user: DiscordOAuthUser;
  scopes: string[];
  csrf_token: string;
  expires_at: string;
}

export interface FeatureCatalogEntry {
  id: string;
  category: string;
  label: string;
  description: string;
  supports_guild_override: boolean;
  global_editable_fields?: string[];
  guild_editable_fields?: string[];
}

export interface FeatureBlocker {
  code: string;
  message: string;
  field?: string;
}

export interface FeatureRecord {
  id: string;
  category: string;
  label: string;
  description: string;
  scope: string;
  supports_guild_override: boolean;
  override_state: string;
  effective_enabled: boolean;
  effective_source: string;
  readiness: string;
  blockers?: FeatureBlocker[];
  details?: Record<string, unknown>;
  editable_fields?: string[];
}

export interface FeatureWorkspace {
  scope: string;
  guild_id?: string;
  features: FeatureRecord[];
}

export interface FeatureCatalogResponse {
  status: string;
  catalog: FeatureCatalogEntry[];
}

export interface FeatureWorkspaceResponse {
  status: string;
  workspace: FeatureWorkspace;
}

export interface FeatureResponse {
  status: string;
  guild_id?: string;
  feature: FeatureRecord;
}

export interface FeaturePatchPayload {
  enabled?: boolean | null;
  channel_id?: string;
  allowed_role_ids?: string[];
  watch_bot?: boolean;
  user_id?: string;
  actor_role_id?: string;
  start_day?: string;
  initial_date?: string;
  config_enabled?: boolean;
  update_interval_mins?: number;
  target_role_id?: string;
  required_role_ids?: string[];
  grace_days?: number;
  scan_interval_mins?: number;
  initial_delay_secs?: number;
  kicks_per_second?: number;
  max_kicks_per_run?: number;
  exempt_role_ids?: string[];
  dry_run?: boolean;
}

export interface DiscordOAuthStatusResponse {
  status: string;
  oauth_configured: boolean;
  authenticated: boolean;
  dashboard_url: string;
  login_url: string;
  user?: DiscordOAuthUser;
  scopes?: string[];
  csrf_token?: string;
  expires_at?: string;
}

export type ControlAuthProbe =
  | { status: "authenticated"; session: AuthSessionResponse }
  | { status: "unauthorized" }
  | { status: "oauth_unavailable" };

export interface ControlApiClientConfig {
  baseUrl: string;
}

export class ControlApiClient {
  private readonly baseUrl: string;
  private csrfToken = "";
  private csrfLoadPromise: Promise<string> | null = null;

  constructor(config: ControlApiClientConfig) {
    this.baseUrl = normalizeBaseUrl(config.baseUrl);
  }

  async getPartnerBoard(guildId: string): Promise<PartnerBoardResponse> {
    return this.request<PartnerBoardResponse>(
      "GET",
      `/v1/guilds/${encodeURIComponent(guildId)}/partner-board`,
    );
  }

  async setPartnerBoardTarget(
    guildId: string,
    payload: EmbedUpdateTargetConfig,
  ): Promise<TargetResponse> {
    return this.request<TargetResponse>(
      "PUT",
      `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/target`,
      payload,
    );
  }

  async setPartnerBoardTemplate(
    guildId: string,
    payload: PartnerBoardTemplateConfig,
  ): Promise<TemplateResponse> {
    return this.request<TemplateResponse>(
      "PUT",
      `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/template`,
      payload,
    );
  }

  async listPartners(guildId: string): Promise<PartnersResponse> {
    return this.request<PartnersResponse>(
      "GET",
      `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/partners`,
    );
  }

  async createPartner(
    guildId: string,
    payload: PartnerEntryConfig,
  ): Promise<PartnerResponse> {
    return this.request<PartnerResponse>(
      "POST",
      `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/partners`,
      payload,
    );
  }

  async updatePartner(
    guildId: string,
    currentName: string,
    partner: PartnerEntryConfig,
  ): Promise<PartnerResponse> {
    return this.request<PartnerResponse>(
      "PUT",
      `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/partners`,
      {
        current_name: currentName,
        partner,
      },
    );
  }

  async deletePartner(guildId: string, name: string): Promise<void> {
    await this.request<Record<string, unknown>>(
      "DELETE",
      `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/partners?name=${encodeURIComponent(name)}`,
    );
  }

  async syncPartnerBoard(guildId: string): Promise<SyncResponse> {
    return this.request<SyncResponse>(
      "POST",
      `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/sync`,
    );
  }

  async listManageableGuilds(): Promise<ManageableGuildsResponse> {
    return this.request<ManageableGuildsResponse>(
      "GET",
      "/auth/guilds/manageable",
    );
  }

  async getFeatureCatalog(): Promise<FeatureCatalogResponse> {
    return this.request<FeatureCatalogResponse>("GET", "/v1/features/catalog");
  }

  async listGlobalFeatures(): Promise<FeatureWorkspaceResponse> {
    return this.request<FeatureWorkspaceResponse>("GET", "/v1/features");
  }

  async getGlobalFeature(featureId: string): Promise<FeatureResponse> {
    return this.request<FeatureResponse>(
      "GET",
      `/v1/features/${encodeURIComponent(featureId)}`,
    );
  }

  async patchGlobalFeature(
    featureId: string,
    payload: FeaturePatchPayload,
  ): Promise<FeatureResponse> {
    return this.request<FeatureResponse>(
      "PATCH",
      `/v1/features/${encodeURIComponent(featureId)}`,
      payload,
    );
  }

  async listGuildFeatures(guildId: string): Promise<FeatureWorkspaceResponse> {
    return this.request<FeatureWorkspaceResponse>(
      "GET",
      `/v1/guilds/${encodeURIComponent(guildId)}/features`,
    );
  }

  async getGuildFeature(
    guildId: string,
    featureId: string,
  ): Promise<FeatureResponse> {
    return this.request<FeatureResponse>(
      "GET",
      `/v1/guilds/${encodeURIComponent(guildId)}/features/${encodeURIComponent(featureId)}`,
    );
  }

  async patchGuildFeature(
    guildId: string,
    featureId: string,
    payload: FeaturePatchPayload,
  ): Promise<FeatureResponse> {
    return this.request<FeatureResponse>(
      "PATCH",
      `/v1/guilds/${encodeURIComponent(guildId)}/features/${encodeURIComponent(featureId)}`,
      payload,
    );
  }

  async getSessionStatus(): Promise<ControlAuthProbe> {
    const url = this.baseUrl === "" ? "/auth/me" : `${this.baseUrl}/auth/me`;
    const response = await fetch(url, {
      method: "GET",
      credentials: "include",
    });

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
    nextPath = "/dashboard/",
  ): Promise<DiscordOAuthStatusResponse> {
    const next = normalizeDashboardNextPath(nextPath);
    const suffix = next === "" ? "" : `?next=${encodeURIComponent(next)}`;
    const url =
      this.baseUrl === ""
        ? `/auth/discord/status${suffix}`
        : `${this.baseUrl}/auth/discord/status${suffix}`;
    const response = await fetch(url, {
      method: "GET",
      credentials: "include",
    });
    if (!response.ok) {
      const text = await response.text();
      throw new Error(
        `Control API GET /auth/discord/status failed: ${response.status} ${response.statusText} - ${text}`.trim(),
      );
    }
    return (await response.json()) as DiscordOAuthStatusResponse;
  }

  private async request<T>(
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

    const response = await fetch(url, {
      method,
      headers,
      credentials: "include",
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });

    if (
      response.status === 403 &&
      retryOnCSRF &&
      requiresCSRFHeader(method)
    ) {
      this.clearCSRFToken();
      return this.request<T>(method, path, body, false);
    }

    if (!response.ok) {
      const text = await response.text();
      throw new Error(
        `Control API ${method} ${path} failed: ${response.status} ${response.statusText} - ${text}`.trim(),
      );
    }

    if (response.status === 204) {
      return {} as T;
    }
    return (await response.json()) as T;
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
          throw new Error("Discord OAuth is not configured on this control server.");
        }
        throw new Error("Unauthorized. Sign in with Discord before changing dashboard settings.");
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

function normalizeBaseUrl(raw: string): string {
  return raw.trim().replace(/\/+$/, "");
}

function normalizeDashboardNextPath(raw: string): string {
  const trimmed = raw.trim();
  if (trimmed === "" || !trimmed.startsWith("/dashboard/")) {
    return "/dashboard/";
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
