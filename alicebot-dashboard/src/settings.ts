import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import type { FeatureSettings, GuildFeatureSettings, GuildSettings, Settings } from "./types";

const defaultExecutablePath =
  process.env.ALICEBOT_PATH ?? "C:\\Users\\alice\\.local\\bin\\alicebot.exe";

export const settingsPath = resolve("./data/settings.json");

const defaultGuilds: GuildSettings[] = [
  {
    id: "123456789012345678",
    name: "Alice Community",
    enabled: true,
    features: {
      monitoring: true,
      automod: true,
      statsChannels: false,
      autoRoleAssignment: false,
      unverifiedPurge: false,
    },
    notificationChannelId: "987654321098765432",
  },
  {
    id: "234567890123456789",
    name: "Dev Lounge",
    enabled: true,
    features: {
      monitoring: false,
      automod: false,
      statsChannels: false,
      autoRoleAssignment: false,
      unverifiedPurge: false,
    },
  },
];

const defaultFeatures: FeatureSettings = {
  services: {
    monitoring: true,
    automod: true,
    commands: true,
    adminCommands: true,
  },
  logging: {
    message: true,
    entryExit: true,
    reaction: true,
    user: true,
    automod: true,
    clean: true,
    moderation: true,
  },
  messageCache: {
    cleanupOnStartup: false,
    deleteOnLog: false,
  },
  presenceWatch: {
    bot: false,
  },
  maintenance: {
    dbCleanup: true,
  },
  safety: {
    botRolePermMirror: true,
  },
  backfill: {
    enabled: true,
  },
};

const defaultGuildFeatures: GuildFeatureSettings = {
  monitoring: true,
  automod: true,
  statsChannels: false,
  autoRoleAssignment: false,
  unverifiedPurge: false,
};

const defaultSettings: Settings = {
  executablePath: defaultExecutablePath,
  guilds: defaultGuilds,
  features: defaultFeatures,
};

const isGuildSettings = (value: GuildSettings): boolean => {
  return (
    typeof value.id === "string" &&
    typeof value.enabled === "boolean" &&
    typeof value.features?.monitoring === "boolean" &&
    typeof value.features?.automod === "boolean" &&
    typeof value.features?.statsChannels === "boolean" &&
    typeof value.features?.autoRoleAssignment === "boolean" &&
    typeof value.features?.unverifiedPurge === "boolean"
  );
};

const boolOrDefault = (value: unknown, fallback: boolean): boolean =>
  typeof value === "boolean" ? value : fallback;

type LegacySettings = Partial<Settings> & {
  monitoringEnabled?: boolean;
  automodEnabled?: boolean;
};

const sanitizeFeatures = (
  input: unknown,
  legacy?: LegacySettings
): FeatureSettings => {
  if (typeof input !== "object" || input === null) {
    return {
      ...defaultFeatures,
      services: {
        ...defaultFeatures.services,
        monitoring: boolOrDefault(
          legacy?.monitoringEnabled,
          defaultFeatures.services.monitoring
        ),
        automod: boolOrDefault(
          legacy?.automodEnabled,
          defaultFeatures.services.automod
        ),
      },
    };
  }

  const data = input as Partial<FeatureSettings>;
  const legacyMonitoring =
    legacy && typeof legacy.monitoringEnabled === "boolean"
      ? legacy.monitoringEnabled
      : undefined;
  const legacyAutomod =
    legacy && typeof legacy.automodEnabled === "boolean"
      ? legacy.automodEnabled
      : undefined;

  return {
    services: {
      monitoring: boolOrDefault(
        data.services?.monitoring,
        legacyMonitoring ?? defaultFeatures.services.monitoring
      ),
      automod: boolOrDefault(
        data.services?.automod,
        legacyAutomod ?? defaultFeatures.services.automod
      ),
      commands: boolOrDefault(
        data.services?.commands,
        defaultFeatures.services.commands
      ),
      adminCommands: boolOrDefault(
        data.services?.adminCommands,
        defaultFeatures.services.adminCommands
      ),
    },
    logging: {
      message: boolOrDefault(data.logging?.message, defaultFeatures.logging.message),
      entryExit: boolOrDefault(
        data.logging?.entryExit,
        defaultFeatures.logging.entryExit
      ),
      reaction: boolOrDefault(
        data.logging?.reaction,
        defaultFeatures.logging.reaction
      ),
      user: boolOrDefault(data.logging?.user, defaultFeatures.logging.user),
      automod: boolOrDefault(data.logging?.automod, defaultFeatures.logging.automod),
      clean: boolOrDefault(data.logging?.clean, defaultFeatures.logging.clean),
      moderation: boolOrDefault(
        data.logging?.moderation,
        defaultFeatures.logging.moderation
      ),
    },
    messageCache: {
      cleanupOnStartup: boolOrDefault(
        data.messageCache?.cleanupOnStartup,
        defaultFeatures.messageCache.cleanupOnStartup
      ),
      deleteOnLog: boolOrDefault(
        data.messageCache?.deleteOnLog,
        defaultFeatures.messageCache.deleteOnLog
      ),
    },
    presenceWatch: {
      bot: boolOrDefault(data.presenceWatch?.bot, defaultFeatures.presenceWatch.bot),
    },
    maintenance: {
      dbCleanup: boolOrDefault(
        data.maintenance?.dbCleanup,
        defaultFeatures.maintenance.dbCleanup
      ),
    },
    safety: {
      botRolePermMirror: boolOrDefault(
        data.safety?.botRolePermMirror,
        defaultFeatures.safety.botRolePermMirror
      ),
    },
    backfill: {
      enabled: boolOrDefault(data.backfill?.enabled, defaultFeatures.backfill.enabled),
    },
  };
};

const sanitizeGuildFeatures = (
  input: unknown,
  legacy?: Partial<GuildSettings> & {
    monitoringEnabled?: boolean;
    automodEnabled?: boolean;
  }
): GuildFeatureSettings => {
  if (typeof input !== "object" || input === null) {
    return {
      ...defaultGuildFeatures,
      monitoring: boolOrDefault(
        legacy?.monitoringEnabled,
        defaultGuildFeatures.monitoring
      ),
      automod: boolOrDefault(
        legacy?.automodEnabled,
        defaultGuildFeatures.automod
      ),
    };
  }

  const data = input as Partial<GuildFeatureSettings>;
  return {
    monitoring: boolOrDefault(
      data.monitoring,
      typeof legacy?.monitoringEnabled === "boolean"
        ? legacy.monitoringEnabled
        : defaultGuildFeatures.monitoring
    ),
    automod: boolOrDefault(
      data.automod,
      typeof legacy?.automodEnabled === "boolean"
        ? legacy.automodEnabled
        : defaultGuildFeatures.automod
    ),
    statsChannels: boolOrDefault(
      data.statsChannels,
      defaultGuildFeatures.statsChannels
    ),
    autoRoleAssignment: boolOrDefault(
      data.autoRoleAssignment,
      defaultGuildFeatures.autoRoleAssignment
    ),
    unverifiedPurge: boolOrDefault(
      data.unverifiedPurge,
      defaultGuildFeatures.unverifiedPurge
    ),
  };
};

const sanitizeGuilds = (guilds: unknown): GuildSettings[] => {
  if (!Array.isArray(guilds)) {
    return defaultGuilds;
  }

  return guilds
    .map((guild) => {
      if (typeof guild !== "object" || guild === null) {
        return null;
      }

      const data = guild as Partial<GuildSettings> & {
        monitoringEnabled?: boolean;
        automodEnabled?: boolean;
      };
      const sanitized: GuildSettings = {
        id: data.id ?? "",
        name: data.name,
        enabled: Boolean(data.enabled),
        features: sanitizeGuildFeatures(data.features, data),
        notificationChannelId: data.notificationChannelId,
      };

      return sanitized;
    })
    .filter((guild): guild is GuildSettings => isGuildSettings(guild));
};

const sanitizeSettings = (input: LegacySettings): Settings => {
  return {
    executablePath: input.executablePath || defaultExecutablePath,
    guilds: sanitizeGuilds(input.guilds),
    features: sanitizeFeatures(input.features, input),
  };
};

export const loadSettings = async (): Promise<Settings> => {
  try {
    const raw = await readFile(settingsPath, "utf-8");
    const parsed = JSON.parse(raw) as Partial<Settings>;
    return sanitizeSettings(parsed);
  } catch {
    await mkdir(dirname(settingsPath), { recursive: true });
    await writeFile(settingsPath, JSON.stringify(defaultSettings, null, 2));
    return { ...defaultSettings };
  }
};

export const saveSettings = async (settings: Settings): Promise<void> => {
  const sanitized = sanitizeSettings(settings);
  await mkdir(dirname(settingsPath), { recursive: true });
  await writeFile(settingsPath, JSON.stringify(sanitized, null, 2));
};

export const validateSettingsPayload = (
  payload: unknown
): { ok: true; settings: Settings } | { ok: false; error: string } => {
  if (typeof payload !== "object" || payload === null) {
    return { ok: false, error: "Payload must be an object." };
  }

  const parsed = payload as Partial<Settings>;
  if (!parsed.executablePath || typeof parsed.executablePath !== "string") {
    return { ok: false, error: "executablePath must be a string." };
  }

  return { ok: true, settings: sanitizeSettings(parsed) };
};
