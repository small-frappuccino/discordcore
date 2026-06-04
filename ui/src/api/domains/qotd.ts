import type { ControlApiClient } from "../client";

export interface QOTDDeck {
  id: string;
  name: string;
  enabled?: boolean;
  channel_id?: string;
}

export interface QOTDPublishScheduleConfig {
  hour_utc?: number;
  minute_utc?: number;
}

export interface QOTDConfig {
  verified_role_id?: string;
  active_deck_id?: string;
  decks?: QOTDDeck[];
  schedule?: QOTDPublishScheduleConfig;
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
  deck_id: string;
  deck_name: string;
  publish_mode: string;
  publish_date_utc: string;
  state: string;
  question_text: string;
  published_at?: string;
  becomes_previous_at: string;
  answers_close_at: string;
  closed_at?: string;
  archived_at?: string;
  thread_id?: string;
  thread_url?: string;
  answer_channel_id?: string;
  answer_channel_url?: string;
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

export interface QOTDSummaryResponse {
  status: string;
  guild_id: string;
  summary: QOTDSummary;
}

export async function getQOTDSummary(client: ControlApiClient, guildId: string): Promise<QOTDSummaryResponse> {
  return client.request<QOTDSummaryResponse>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/qotd`,
  );
}

export async function getQOTDSettings(client: ControlApiClient, guildId: string): Promise<QOTDSettingsResponse> {
  return client.request<QOTDSettingsResponse>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/qotd/settings`,
  );
}

export async function updateQOTDSettings(
  client: ControlApiClient,
  guildId: string,
  payload: QOTDConfig,
): Promise<QOTDSettingsResponse> {
  return client.request<QOTDSettingsResponse>(
    "PUT",
    `/v1/guilds/${encodeURIComponent(guildId)}/qotd/settings`,
    payload,
  );
}
