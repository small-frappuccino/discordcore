import type { ControlApiClient } from "../client";

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

export interface AutoAssignmentConfig {
  enabled?: boolean;
  target_role?: string;
  required_roles?: string[];
}

export interface GuildRolesSettingsSection {
  allowed?: string[];
  dashboard_read?: string[];
  dashboard_write?: string[];
  auto_assignment?: AutoAssignmentConfig;
  booster_role?: string;
  mute_role?: string;
}



export interface GuildSettingsWorkspace {
  scope: string;
  guild_id: string;
  bot_instance_id?: string;
  available_bot_instance_ids?: string[];
  sections: {
    bot_instance_tokens: Record<string, string>;
    roles: GuildRolesSettingsSection;
  };
}

export interface GuildSettingsWorkspaceResponse {
  status: string;
  workspace: GuildSettingsWorkspace;
}

export interface GuildListRequestOptions {
  fresh?: boolean;
}

export interface GuildChannelOptionsRequestOptions {
  domain?: string;
}

function buildGuildListPath(path: string, options: GuildListRequestOptions) {
  if (!options.fresh) {
    return path;
  }
  return `${path}?fresh=1`;
}

export async function listAccessibleGuilds(
  client: ControlApiClient,
  options: GuildListRequestOptions = {},
): Promise<AccessibleGuildsResponse> {
  return client.request<AccessibleGuildsResponse>(
    "GET",
    buildGuildListPath("/auth/guilds/access", options),
  );
}

export async function listManageableGuilds(
  client: ControlApiClient,
  options: GuildListRequestOptions = {},
): Promise<ManageableGuildsResponse> {
  return client.request<ManageableGuildsResponse>(
    "GET",
    buildGuildListPath("/auth/guilds/manageable", options),
  );
}

export async function getGuildSettings(
  client: ControlApiClient,
  guildId: string,
): Promise<GuildSettingsWorkspaceResponse> {
  return client.request<GuildSettingsWorkspaceResponse>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/settings`,
  );
}

export async function updateGuildSettings(
  client: ControlApiClient,
  guildId: string,
  payload: {
    bot_instance_tokens?: Record<string, string>;
    roles?: GuildRolesSettingsSection;
  },
): Promise<GuildSettingsWorkspaceResponse> {
  return client.request<GuildSettingsWorkspaceResponse>(
    "PUT",
    `/v1/guilds/${encodeURIComponent(guildId)}/settings`,
    payload,
  );
}

export async function listGuildRoleOptions(
  client: ControlApiClient,
  guildId: string,
): Promise<GuildRoleOptionsResponse> {
  return client.request<GuildRoleOptionsResponse>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/role-options`,
  );
}

export async function listGuildChannelOptions(
  client: ControlApiClient,
  guildId: string,
  options: GuildChannelOptionsRequestOptions = {},
): Promise<GuildChannelOptionsResponse> {
  const params = new URLSearchParams();
  const domain = options.domain?.trim() ?? "";
  if (domain !== "") {
    params.set("domain", domain);
  }

  const suffix = params.size > 0 ? `?${params.toString()}` : "";
  return client.request<GuildChannelOptionsResponse>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/channel-options${suffix}`,
  );
}

export async function listGuildMemberOptions(
  client: ControlApiClient,
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
  return client.request<GuildMemberOptionsResponse>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/member-options${suffix}`,
  );
}
