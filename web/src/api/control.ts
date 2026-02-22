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

export interface ControlApiClientConfig {
  baseUrl: string;
  bearerToken: string;
}

export class ControlApiClient {
  private readonly baseUrl: string;
  private readonly bearerToken: string;

  constructor(config: ControlApiClientConfig) {
    this.baseUrl = normalizeBaseUrl(config.baseUrl);
    this.bearerToken = config.bearerToken.trim();
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

  private async request<T>(
    method: "GET" | "POST" | "PUT" | "DELETE",
    path: string,
    body?: unknown,
  ): Promise<T> {
    if (this.bearerToken === "") {
      throw new Error("Control API bearer token is required");
    }
    const url = this.baseUrl === "" ? path : `${this.baseUrl}${path}`;

    const headers = new Headers();
    headers.set("Authorization", `Bearer ${this.bearerToken}`);
    if (body !== undefined) {
      headers.set("Content-Type", "application/json");
    }

    const response = await fetch(url, {
      method,
      headers,
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });

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
}

function normalizeBaseUrl(raw: string): string {
  return raw.trim().replace(/\/+$/, "");
}
