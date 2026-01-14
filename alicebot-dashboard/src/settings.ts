import { mkdir, readFile, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import type { GuildSettings, Settings } from "./types";

const defaultExecutablePath =
  process.env.ALICEBOT_PATH ?? "C:\\Users\\alice\\.local\\bin\\alicebot.exe";

export const settingsPath = resolve("./data/settings.json");

const defaultGuilds: GuildSettings[] = [
  {
    id: "123456789012345678",
    name: "Alice Community",
    enabled: true,
    monitoringEnabled: true,
    automodEnabled: true,
    notificationChannelId: "987654321098765432",
  },
  {
    id: "234567890123456789",
    name: "Dev Lounge",
    enabled: true,
    monitoringEnabled: false,
    automodEnabled: false,
  },
];

const defaultSettings: Settings = {
  executablePath: defaultExecutablePath,
  guilds: defaultGuilds,
  automodEnabled: true,
  monitoringEnabled: true,
};

const isGuildSettings = (value: GuildSettings): boolean => {
  return (
    typeof value.id === "string" &&
    typeof value.enabled === "boolean" &&
    typeof value.monitoringEnabled === "boolean" &&
    typeof value.automodEnabled === "boolean"
  );
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

      const data = guild as Partial<GuildSettings>;
      const sanitized: GuildSettings = {
        id: data.id ?? "",
        name: data.name,
        enabled: Boolean(data.enabled),
        monitoringEnabled: Boolean(data.monitoringEnabled),
        automodEnabled: Boolean(data.automodEnabled),
        notificationChannelId: data.notificationChannelId,
      };

      return sanitized;
    })
    .filter((guild): guild is GuildSettings => isGuildSettings(guild));
};

const sanitizeSettings = (input: Partial<Settings>): Settings => {
  return {
    executablePath: input.executablePath || defaultExecutablePath,
    guilds: sanitizeGuilds(input.guilds),
    automodEnabled:
      typeof input.automodEnabled === "boolean"
        ? input.automodEnabled
        : defaultSettings.automodEnabled,
    monitoringEnabled:
      typeof input.monitoringEnabled === "boolean"
        ? input.monitoringEnabled
        : defaultSettings.monitoringEnabled,
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
