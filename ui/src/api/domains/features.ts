import type { ControlApiClient } from "../client";

export type FeatureAreaID =
  | "commands"
  | "moderation"
  | "logging"
  | "roles"
  | "maintenance"
  | "stats";

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

export interface FeatureStatsChannelDetail {
  channel_id?: string;
  label?: string;
  name_template?: string;
  member_type?: string;
  role_id?: string;
}

export interface FeatureDetails {
  mode?: string;
  role_id?: string;
  channel_id?: string;
  allowed_role_ids?: string[];
  allowed_role_count?: number;
  runtime_enabled?: boolean;
  watch_bot?: boolean;
  user_id?: string;
  actor_role_id?: string;
  runtime_disabled?: boolean;
  start_day?: string;
  initial_date?: string;
  config_enabled?: boolean;
  update_interval_mins?: number;
  configured_channel_count?: number;
  channels?: FeatureStatsChannelDetail[];
  target_role_id?: string;
  required_role_ids?: string[];
  required_role_count?: number;
  booster_role_id?: string;
  level_role_id?: string;
  requires_channel?: boolean;
  required_intents_mask?: number;
  required_permissions_mask?: number;
  validate_channel_permissions?: boolean;
  exclusive_moderation_channel?: boolean;
  runtime_toggle_path?: string;
  [key: string]: unknown;
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
  config_version?: number;
  readiness: string;
  blockers?: FeatureBlocker[];
  details?: FeatureDetails;
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
  config_version?: number;
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

export async function getFeatureCatalog(client: ControlApiClient): Promise<FeatureCatalogResponse> {
  return client.request<FeatureCatalogResponse>("GET", "/v1/features/catalog");
}

export async function listGlobalFeatures(client: ControlApiClient): Promise<FeatureWorkspaceResponse> {
  return client.request<FeatureWorkspaceResponse>("GET", "/v1/features");
}

export async function getGlobalFeature(client: ControlApiClient, featureId: string): Promise<FeatureResponse> {
  return client.request<FeatureResponse>(
    "GET",
    `/v1/features/${encodeURIComponent(featureId)}`,
  );
}

export async function patchGlobalFeature(
  client: ControlApiClient,
  featureId: string,
  payload: FeaturePatchPayload,
): Promise<FeatureResponse> {
  return client.request<FeatureResponse>(
    "PATCH",
    `/v1/features/${encodeURIComponent(featureId)}`,
    payload,
  );
}

export async function listGuildFeatures(client: ControlApiClient, guildId: string): Promise<FeatureWorkspaceResponse> {
  return client.request<FeatureWorkspaceResponse>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/features`,
  );
}

export async function getGuildFeature(
  client: ControlApiClient,
  guildId: string,
  featureId: string,
): Promise<FeatureResponse> {
  return client.request<FeatureResponse>(
    "GET",
    `/v1/guilds/${encodeURIComponent(guildId)}/features/${encodeURIComponent(featureId)}`,
  );
}

export async function patchGuildFeature(
  client: ControlApiClient,
  guildId: string,
  featureId: string,
  originalFeature: FeatureRecord | undefined,
  payload: FeaturePatchPayload,
): Promise<FeatureResponse> {
  const { delay } = await import("../client");
  let attempt = 0;
  const maxAttempts = 4;
  const currentPayload = { ...payload };

  while (true) {
    try {
      return await client.request<FeatureResponse>(
        "PATCH",
        `/v1/guilds/${encodeURIComponent(guildId)}/features/${encodeURIComponent(featureId)}`,
        currentPayload,
      );
    } catch (error: unknown) {
      const err = error as { message?: string };
      if (!err.message?.includes("412") && !err.message?.includes("428")) {
        throw error;
      }
      if (attempt >= maxAttempts - 1) {
        throw new Error(`Failed to save feature due to concurrent modifications after ${maxAttempts} attempts. Please refresh the page and try again.`);
      }

      attempt++;
      // Exponential backoff and randomized network jitter
      const delayMs = Math.pow(2, attempt) * 100 + Math.random() * 50;
      await delay(delayMs);

      // State-refresh I/O call
      const latestResponse = await getGuildFeature(client, guildId, featureId);
      const latestFeature = latestResponse.feature;

      // Explicitly check field-level mutation collisions
      let collision = false;
      if (originalFeature) {
        if (currentPayload.enabled !== undefined && latestFeature.effective_enabled !== originalFeature.effective_enabled) {
          collision = true;
        }
        if (currentPayload.channel_id !== undefined && latestFeature.details?.channel_id !== originalFeature.details?.channel_id) {
          collision = true;
        }
        if (currentPayload.role_id !== undefined && latestFeature.details?.role_id !== originalFeature.details?.role_id) {
          collision = true;
        }
        if (currentPayload.allowed_role_ids !== undefined && JSON.stringify(latestFeature.details?.allowed_role_ids) !== JSON.stringify(originalFeature.details?.allowed_role_ids)) {
          collision = true;
        }
        // ... (we can add more fields if needed, but enabled, channel_id, role_id, allowed_role_ids are the main ones)
      }

      if (collision) {
        throw new Error("Concurrent modification detected on the same fields. Please refresh and try again. (Lost Update)");
      }

      if (latestFeature.config_version !== undefined) {
        currentPayload.config_version = latestFeature.config_version;
      }
    }
  }
}
