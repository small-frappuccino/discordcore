import type { ControlApiClient } from "../client";

export interface CustomEmbedFieldConfig {
  name: string;
  value: string;
  inline?: boolean;
}

export interface CustomEmbedPostingConfig {
  channel_id: string;
  message_id?: string;
  webhook_url?: string;
}

export interface CustomEmbedConfig {
  key: string;
  title?: string;
  description?: string;
  color?: number;
  author_name?: string;
  author_icon_url?: string;
  footer_text?: string;
  footer_icon_url?: string;
  image_url?: string;
  thumbnail_url?: string;
  fields?: CustomEmbedFieldConfig[];
  postings?: CustomEmbedPostingConfig[];
}

export async function getCustomEmbeds(client: ControlApiClient, guildId: string): Promise<CustomEmbedConfig[]> {
  return client.request<CustomEmbedConfig[]>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/embeds`
  );
}

export async function getCustomEmbed(client: ControlApiClient, guildId: string, key: string): Promise<CustomEmbedConfig> {
  return client.request<CustomEmbedConfig>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/embeds/${encodeURIComponent(key)}`
  );
}

export async function putCustomEmbed(
  client: ControlApiClient,
  guildId: string,
  key: string,
  payload: CustomEmbedConfig
): Promise<CustomEmbedConfig> {
  return client.request<CustomEmbedConfig>(
    "PUT",
    `/v1/guilds/${encodeURIComponent(guildId)}/embeds/${encodeURIComponent(key)}`,
    payload
  );
}

export async function deleteCustomEmbed(client: ControlApiClient, guildId: string, key: string): Promise<boolean> {
  return client.request<boolean>(
    "DELETE",
    `/v1/guilds/${encodeURIComponent(guildId)}/embeds/${encodeURIComponent(key)}`
  );
}
