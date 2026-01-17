export type LogLevel = "INFO" | "WARN" | "ERROR" | "DEBUG";

export type EventLog = {
  ts: string;
  level: LogLevel;
  category: string;
  message: string;
  guildId?: string;
  userId?: string;
  channelId?: string;
  meta?: Record<string, string>;
  stream: "stdout" | "stderr";
};

export type FeatureSettings = {
  services: {
    monitoring: boolean;
    automod: boolean;
    commands: boolean;
    adminCommands: boolean;
  };
  logging: {
    message: boolean;
    entryExit: boolean;
    reaction: boolean;
    user: boolean;
    automod: boolean;
    clean: boolean;
    moderation: boolean;
  };
  messageCache: {
    cleanupOnStartup: boolean;
    deleteOnLog: boolean;
  };
  presenceWatch: {
    bot: boolean;
  };
  maintenance: {
    dbCleanup: boolean;
  };
  safety: {
    botRolePermMirror: boolean;
  };
  backfill: {
    enabled: boolean;
  };
};

export type GuildFeatureSettings = {
  monitoring: boolean;
  automod: boolean;
  statsChannels: boolean;
  autoRoleAssignment: boolean;
  unverifiedPurge: boolean;
};

export type ProcessStatus = {
  running: boolean;
  pid?: number;
  startedAt?: string;
  lastExitCode?: number;
  lastExitSignal?: string;
  executablePath: string;
};

export type BotStatus = {
  connected: boolean;
  username: string;
  guildCount: number;
  uptimeSeconds: number;
};

export type GuildSettings = {
  id: string;
  name?: string;
  enabled: boolean;
  features: GuildFeatureSettings;
  notificationChannelId?: string;
};

export type Settings = {
  executablePath: string;
  guilds: GuildSettings[];
  features: FeatureSettings;
};
