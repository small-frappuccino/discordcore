import type { ControlApiClient } from "../client";

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

export async function getPartnerBoard(client: ControlApiClient, guildId: string): Promise<PartnerBoardResponse> {
  return client.request<PartnerBoardResponse>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/partner-board`,
  );
}

export async function setPartnerBoardTarget(
  client: ControlApiClient,
  guildId: string,
  payload: EmbedUpdateTargetConfig,
): Promise<TargetResponse> {
  return client.request<TargetResponse>(
    "PUT",
    `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/target`,
    payload,
  );
}

export async function setPartnerBoardTemplate(
  client: ControlApiClient,
  guildId: string,
  payload: PartnerBoardTemplateConfig,
): Promise<TemplateResponse> {
  return client.request<TemplateResponse>(
    "PUT",
    `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/template`,
    payload,
  );
}

export async function listPartners(client: ControlApiClient, guildId: string): Promise<PartnersResponse> {
  return client.request<PartnersResponse>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/partners`,
  );
}

export async function createPartner(
  client: ControlApiClient,
  guildId: string,
  payload: PartnerEntryConfig,
): Promise<PartnerResponse> {
  return client.request<PartnerResponse>(
    "POST",
    `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/partners`,
    payload,
  );
}

export async function updatePartner(
  client: ControlApiClient,
  guildId: string,
  currentName: string,
  partner: PartnerEntryConfig,
): Promise<PartnerResponse> {
  return client.request<PartnerResponse>(
    "PUT",
    `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/partners`,
    {
      current_name: currentName,
      partner,
    },
  );
}

export async function deletePartner(client: ControlApiClient, guildId: string, name: string): Promise<void> {
  await client.request<Record<string, unknown>>(
    "DELETE",
    `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/partners?name=${encodeURIComponent(name)}`,
  );
}

export async function syncPartnerBoard(client: ControlApiClient, guildId: string): Promise<SyncResponse> {
  return client.request<SyncResponse>(
    "POST",
    `/v1/guilds/${encodeURIComponent(guildId)}/partner-board/sync`,
  );
}
