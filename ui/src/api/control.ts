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

export type QOTDQuestionStatus =
  | "draft"
  | "ready"
  | "reserved"
  | "used"
  | "disabled";

export interface QOTDDeck {
  id: string;
  name: string;
  enabled?: boolean;
  forum_channel_id?: string;
}

export interface QOTDCollectorConfig {
  source_channel_id?: string;
  author_ids?: string[];
  title_patterns?: string[];
  start_date?: string;
}

export interface QOTDConfig {
  active_deck_id?: string;
  decks?: QOTDDeck[];
  collector?: QOTDCollectorConfig;
}

export interface QOTDQuestion {
  id: number;
  deck_id: string;
  body: string;
  status: QOTDQuestionStatus;
  queue_position: number;
  created_by?: string;
  scheduled_for_date_utc?: string;
  used_at?: string;
  created_at: string;
  updated_at: string;
}

export interface QOTDQuestionCounts {
  total: number;
  draft: number;
  ready: number;
  reserved: number;
  used: number;
  disabled: number;
}

export interface QOTDDeckSummary {
  id: string;
  name: string;
  enabled: boolean;
  counts: QOTDQuestionCounts;
  cards_remaining: number;
  is_active: boolean;
  can_publish: boolean;
}

export interface QOTDOfficialPost {
  id: number;
  deck_id: string;
  deck_name: string;
  question_id: number;
  publish_mode: string;
  publish_date_utc: string;
  state: string;
  forum_channel_id: string;
  question_list_thread_id?: string;
  question_list_entry_message_id?: string;
  discord_thread_id?: string;
  discord_starter_message_id?: string;
  answer_channel_id?: string;
  question_text_snapshot: string;
  published_at?: string;
  grace_until: string;
  archive_at: string;
  closed_at?: string;
  archived_at?: string;
  post_url?: string;
}

export interface QOTDSummary {
  settings: QOTDConfig;
  counts: QOTDQuestionCounts;
  decks: QOTDDeckSummary[];
  current_publish_date_utc: string;
  published_for_current_slot: boolean;
  current_post?: QOTDOfficialPost;
  previous_post?: QOTDOfficialPost;
}

export interface QOTDSettingsResponse {
  status: string;
  guild_id: string;
  settings: QOTDConfig;
}

export interface QOTDQuestionsResponse {
  status: string;
  guild_id: string;
  questions: QOTDQuestion[];
}

export interface QOTDQuestionResponse {
  status: string;
  guild_id: string;
  question: QOTDQuestion;
}

export interface QOTDSummaryResponse {
  status: string;
  guild_id: string;
  summary: QOTDSummary;
}

export interface QOTDPublishResult {
  post_url?: string;
  question: QOTDQuestion;
  official_post: QOTDOfficialPost;
}

export interface QOTDQuestionMutation {
  deck_id?: string;
  body: string;
  status: QOTDQuestionStatus;
}

export interface QOTDPublishResponse {
  status: string;
  guild_id: string;
  result: QOTDPublishResult;
}

export interface QOTDCollectedQuestion {
  id: number;
  source_channel_id: string;
  source_message_id: string;
  source_author_id?: string;
  source_author_name?: string;
  source_created_at: string;
  embed_title: string;
  question_text: string;
  created_at: string;
  updated_at: string;
}

export interface QOTDCollectorSummary {
  total_questions: number;
  recent_questions: QOTDCollectedQuestion[];
}

export interface QOTDCollectorRunResult {
  scanned_messages: number;
  matched_messages: number;
  new_questions: number;
  total_questions: number;
}

export interface QOTDCollectorSummaryResponse {
  status: string;
  guild_id: string;
  summary: QOTDCollectorSummary;
}

export interface QOTDCollectorRunResponse {
  status: string;
  guild_id: string;
  result: QOTDCollectorRunResult;
  summary: QOTDCollectorSummary;
}

export type DashboardGuildAccessLevel = "read" | "write";

export interface AccessibleGuild {
  id: string;
  name: string;
  icon?: string;
  bot_present?: boolean;
  owner: boolean;
  permissions: number;
  access_level: DashboardGuildAccessLevel;
}

export interface AccessibleGuildsResponse {
  status: string;
  count: number;
  guilds: AccessibleGuild[];
}

export type ManageableGuild = AccessibleGuild;
export type ManageableGuildsResponse = AccessibleGuildsResponse;

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
  area?: FeatureAreaID;
  tags?: string[];
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
  area?: FeatureAreaID;
  tags?: string[];
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

export type FeatureAreaID =
  | "commands"
  | "moderation"
  | "logging"
  | "roles"
  | "maintenance"
  | "stats";

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
  role_id?: string;
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

export interface GuildRoleOption {
  id: string;
  name: string;
  position: number;
  managed: boolean;
  is_default: boolean;
}

export interface GuildChannelOption {
  id: string;
  name: string;
  display_name: string;
  kind: string;
  supports_message_route: boolean;
}

export interface GuildRoleOptionsResponse {
  status: string;
  guild_id: string;
  roles: GuildRoleOption[];
}

export interface GuildChannelOptionsResponse {
  status: string;
  guild_id: string;
  channels: GuildChannelOption[];
}

export interface GuildMemberOption {
  id: string;
  display_name: string;
  username: string;
  bot: boolean;
}

export interface GuildMemberOptionsResponse {
  status: string;
  guild_id: string;
  members: GuildMemberOption[];
}

export interface GuildRolesSettingsSection {
  allowed?: string[];
  dashboard_read?: string[];
  dashboard_write?: string[];
}

export interface GuildSettingsWorkspace {
  scope: string;
  guild_id: string;
  sections: {
    roles: GuildRolesSettingsSection;
  };
}

export interface GuildSettingsWorkspaceResponse {
  status: string;
  workspace: GuildSettingsWorkspace;
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

interface GuildListRequestOptions {
  fresh?: boolean;
}

const transientGetRetryStatuses = new Set([502, 504]);
const transientGetRetryDelaysMs = [80, 160];

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

  async getQOTDSummary(guildId: string): Promise<QOTDSummaryResponse> {
    return this.request<QOTDSummaryResponse>(
      "GET",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd`,
    );
  }

  async getQOTDSettings(guildId: string): Promise<QOTDSettingsResponse> {
    return this.request<QOTDSettingsResponse>(
      "GET",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd/settings`,
    );
  }

  async updateQOTDSettings(
    guildId: string,
    payload: QOTDConfig,
  ): Promise<QOTDSettingsResponse> {
    return this.request<QOTDSettingsResponse>(
      "PUT",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd/settings`,
      payload,
    );
  }

  async listQOTDQuestions(
    guildId: string,
    deckId?: string,
  ): Promise<QOTDQuestionsResponse> {
    const params = new URLSearchParams();
    if (deckId && deckId.trim() !== "") {
      params.set("deck_id", deckId.trim());
    }
    return this.request<QOTDQuestionsResponse>(
      "GET",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd/questions${params.size > 0 ? `?${params.toString()}` : ""}`,
    );
  }

  async createQOTDQuestion(
    guildId: string,
    payload: QOTDQuestionMutation,
  ): Promise<QOTDQuestionResponse> {
    return this.request<QOTDQuestionResponse>(
      "POST",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd/questions`,
      payload,
    );
  }

  async createQOTDQuestionsBatch(
    guildId: string,
    payload: { questions: QOTDQuestionMutation[] },
  ): Promise<QOTDQuestionsResponse> {
    return this.request<QOTDQuestionsResponse>(
      "POST",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd/questions/batch`,
      payload,
    );
  }

  async updateQOTDQuestion(
    guildId: string,
    questionId: number,
    payload: QOTDQuestionMutation,
  ): Promise<QOTDQuestionResponse> {
    return this.request<QOTDQuestionResponse>(
      "PUT",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd/questions/${questionId}`,
      payload,
    );
  }

  async deleteQOTDQuestion(guildId: string, questionId: number): Promise<void> {
    await this.request<Record<string, unknown>>(
      "DELETE",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd/questions/${questionId}`,
    );
  }

  async reorderQOTDQuestions(
    guildId: string,
    deckId: string,
    orderedIDs: number[],
  ): Promise<QOTDQuestionsResponse> {
    return this.request<QOTDQuestionsResponse>(
      "POST",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd/questions/reorder`,
      {
        deck_id: deckId,
        ordered_ids: orderedIDs,
      },
    );
  }

  async publishQOTDNow(guildId: string): Promise<QOTDPublishResponse> {
    return this.request<QOTDPublishResponse>(
      "POST",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd/actions/publish-now`,
    );
  }

  async getQOTDCollectorSummary(
    guildId: string,
  ): Promise<QOTDCollectorSummaryResponse> {
    return this.request<QOTDCollectorSummaryResponse>(
      "GET",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd/collector`,
    );
  }

  async runQOTDCollector(guildId: string): Promise<QOTDCollectorRunResponse> {
    return this.request<QOTDCollectorRunResponse>(
      "POST",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd/collector/collect`,
    );
  }

  async downloadQOTDCollectorExport(
    guildId: string,
  ): Promise<{ filename: string; text: string }> {
    const path = `/v1/guilds/${encodeURIComponent(guildId)}/qotd/collector/export`;
    const url = this.baseUrl === "" ? path : `${this.baseUrl}${path}`;
    const response = await this.fetchWithTransientGetRetry(url);
    if (!response.ok) {
      const text = await response.text();
      throw new Error(
        `Control API GET ${path} failed: ${response.status} ${response.statusText} - ${text}`.trim(),
      );
    }

    const disposition = response.headers.get("Content-Disposition") ?? "";
    const filenameMatch = disposition.match(/filename="?([^";]+)"?/i);
    return {
      filename: filenameMatch?.[1]?.trim() || "qotd-collected-questions.txt",
      text: await response.text(),
    };
  }

  async reconcileQOTD(guildId: string): Promise<QOTDSummaryResponse> {
    return this.request<QOTDSummaryResponse>(
      "POST",
      `/v1/guilds/${encodeURIComponent(guildId)}/qotd/actions/reconcile`,
    );
  }

  async listAccessibleGuilds(
    options: GuildListRequestOptions = {},
  ): Promise<AccessibleGuildsResponse> {
    return this.request<AccessibleGuildsResponse>(
      "GET",
      buildGuildListPath("/auth/guilds/access", options),
    );
  }

  async listManageableGuilds(
    options: GuildListRequestOptions = {},
  ): Promise<ManageableGuildsResponse> {
    return this.request<ManageableGuildsResponse>(
      "GET",
      buildGuildListPath("/auth/guilds/manageable", options),
    );
  }

  async getGuildSettings(
    guildId: string,
  ): Promise<GuildSettingsWorkspaceResponse> {
    return this.request<GuildSettingsWorkspaceResponse>(
      "GET",
      `/v1/guilds/${encodeURIComponent(guildId)}/settings`,
    );
  }

  async updateGuildSettings(
    guildId: string,
    payload: {
      roles?: GuildRolesSettingsSection;
    },
  ): Promise<GuildSettingsWorkspaceResponse> {
    return this.request<GuildSettingsWorkspaceResponse>(
      "PUT",
      `/v1/guilds/${encodeURIComponent(guildId)}/settings`,
      payload,
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

  async listGuildRoleOptions(
    guildId: string,
  ): Promise<GuildRoleOptionsResponse> {
    return this.request<GuildRoleOptionsResponse>(
      "GET",
      `/v1/guilds/${encodeURIComponent(guildId)}/role-options`,
    );
  }

  async listGuildChannelOptions(
    guildId: string,
  ): Promise<GuildChannelOptionsResponse> {
    return this.request<GuildChannelOptionsResponse>(
      "GET",
      `/v1/guilds/${encodeURIComponent(guildId)}/channel-options`,
    );
  }

  async listGuildMemberOptions(
    guildId: string,
    options: {
      query?: string;
      selectedId?: string;
      limit?: number;
    } = {},
  ): Promise<GuildMemberOptionsResponse> {
    const params = new URLSearchParams();
    const query = options.query?.trim() ?? "";
    const selectedId = options.selectedId?.trim() ?? "";
    if (query !== "") {
      params.set("query", query);
    }
    if (selectedId !== "") {
      params.set("selected_id", selectedId);
    }
    if (
      typeof options.limit === "number" &&
      Number.isFinite(options.limit) &&
      options.limit > 0
    ) {
      params.set("limit", String(Math.trunc(options.limit)));
    }

    const suffix = params.size > 0 ? `?${params.toString()}` : "";
    return this.request<GuildMemberOptionsResponse>(
      "GET",
      `/v1/guilds/${encodeURIComponent(guildId)}/member-options${suffix}`,
    );
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

function delay(ms: number) {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}

function buildGuildListPath(path: string, options: GuildListRequestOptions) {
  if (!options.fresh) {
    return path;
  }
  return `${path}?fresh=1`;
}
