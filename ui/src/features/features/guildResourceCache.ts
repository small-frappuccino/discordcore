import type {
  ControlApiClient,
  FeatureRecord,
  FeatureWorkspace,
  FeatureWorkspaceResponse,
  GuildChannelOption,
  GuildChannelOptionsResponse,
  GuildRoleOption,
  GuildRoleOptionsResponse,
} from "../../api/control";

type FeatureWorkspaceScope = "global" | "guild";

const resourceCacheTTL = 60_000;

interface ResourceCacheEntry<T> {
  value: T | null;
  promise: Promise<T> | null;
  updatedAt: number;
}

const featureWorkspaceCache = new Map<
  string,
  ResourceCacheEntry<FeatureWorkspaceResponse>
>();
const guildChannelOptionsCache = new Map<
  string,
  ResourceCacheEntry<GuildChannelOptionsResponse>
>();
const guildRoleOptionsCache = new Map<
  string,
  ResourceCacheEntry<GuildRoleOptionsResponse>
>();

function normalizeGuildID(guildID: string) {
  return guildID.trim();
}

function featureWorkspaceCacheKey(
  baseUrl: string,
  scope: FeatureWorkspaceScope,
  guildID: string,
) {
  const normalizedGuildID = scope === "guild" ? normalizeGuildID(guildID) : "";
  return `${baseUrl}::${scope}::${normalizedGuildID}`;
}

function guildScopedCacheKey(baseUrl: string, guildID: string) {
  return `${baseUrl}::${normalizeGuildID(guildID)}`;
}

function createCacheEntry<T>(): ResourceCacheEntry<T> {
  return {
    value: null,
    promise: null,
    updatedAt: 0,
  };
}

function isFresh<T>(entry: ResourceCacheEntry<T>) {
  return entry.value !== null && Date.now() - entry.updatedAt < resourceCacheTTL;
}

function loadCachedResource<T>(
  cache: Map<string, ResourceCacheEntry<T>>,
  key: string,
  loader: () => Promise<T>,
  force = false,
) {
  const entry = cache.get(key) ?? createCacheEntry<T>();
  cache.set(key, entry);

  if (!force && isFresh(entry) && entry.value !== null) {
    return Promise.resolve(entry.value);
  }
  if (entry.promise !== null) {
    return entry.promise;
  }

  const promise = loader()
    .then((value) => {
      entry.value = value;
      entry.updatedAt = Date.now();
      return value;
    })
    .finally(() => {
      if (entry.promise === promise) {
        entry.promise = null;
      }
    });
  entry.promise = promise;
  return promise;
}

export function resetGuildResourceCache() {
  featureWorkspaceCache.clear();
  guildChannelOptionsCache.clear();
  guildRoleOptionsCache.clear();
}

export function peekFeatureWorkspace(
  baseUrl: string,
  scope: FeatureWorkspaceScope,
  guildID: string,
): FeatureWorkspace | null {
  const entry = featureWorkspaceCache.get(
    featureWorkspaceCacheKey(baseUrl, scope, guildID),
  );
  return entry?.value?.workspace ?? null;
}

export function peekGuildChannelOptions(
  baseUrl: string,
  guildID: string,
): GuildChannelOption[] {
  return (
    guildChannelOptionsCache.get(guildScopedCacheKey(baseUrl, guildID))?.value
      ?.channels ?? []
  );
}

export function peekGuildRoleOptions(
  baseUrl: string,
  guildID: string,
): GuildRoleOption[] {
  return (
    guildRoleOptionsCache.get(guildScopedCacheKey(baseUrl, guildID))?.value
      ?.roles ?? []
  );
}

export async function loadFeatureWorkspace(
  client: ControlApiClient,
  baseUrl: string,
  scope: FeatureWorkspaceScope,
  guildID: string,
  options: {
    force?: boolean;
  } = {},
) {
  const response = await loadCachedResource(
    featureWorkspaceCache,
    featureWorkspaceCacheKey(baseUrl, scope, guildID),
    () =>
      scope === "guild"
        ? client.listGuildFeatures(normalizeGuildID(guildID))
        : client.listGlobalFeatures(),
    options.force,
  );
  return response.workspace;
}

export async function loadGuildChannelOptions(
  client: ControlApiClient,
  baseUrl: string,
  guildID: string,
  options: {
    force?: boolean;
  } = {},
) {
  const response = await loadCachedResource(
    guildChannelOptionsCache,
    guildScopedCacheKey(baseUrl, guildID),
    () => client.listGuildChannelOptions(normalizeGuildID(guildID)),
    options.force,
  );
  return response.channels;
}

export async function loadGuildRoleOptions(
  client: ControlApiClient,
  baseUrl: string,
  guildID: string,
  options: {
    force?: boolean;
  } = {},
) {
  const response = await loadCachedResource(
    guildRoleOptionsCache,
    guildScopedCacheKey(baseUrl, guildID),
    () => client.listGuildRoleOptions(normalizeGuildID(guildID)),
    options.force,
  );
  return response.roles;
}

export async function prefetchGuildDashboardResources(
  client: ControlApiClient,
  baseUrl: string,
  guildID: string,
) {
  const normalizedGuildID = normalizeGuildID(guildID);
  if (normalizedGuildID === "") {
    return;
  }

  await Promise.allSettled([
    loadFeatureWorkspace(client, baseUrl, "guild", normalizedGuildID),
    loadGuildChannelOptions(client, baseUrl, normalizedGuildID),
    loadGuildRoleOptions(client, baseUrl, normalizedGuildID),
  ]);
}

export function updateCachedGuildFeatureRecord(
  baseUrl: string,
  guildID: string,
  feature: FeatureRecord,
) {
  const key = featureWorkspaceCacheKey(baseUrl, "guild", guildID);
  const entry = featureWorkspaceCache.get(key);
  if (entry === undefined || entry.value === null) {
    return;
  }
  const response = entry.value;

  const nextFeatures = response.workspace.features.map((currentFeature) =>
    currentFeature.id === feature.id ? feature : currentFeature,
  );
  const featureExists = nextFeatures.some(
    (currentFeature) => currentFeature.id === feature.id,
  );
  if (!featureExists) {
    return;
  }

  entry.value = {
    ...response,
    workspace: {
      ...response.workspace,
      features: nextFeatures,
    },
  };
  entry.updatedAt = Date.now();
}
