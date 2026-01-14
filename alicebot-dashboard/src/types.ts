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
  monitoringEnabled: boolean;
  automodEnabled: boolean;
  notificationChannelId?: string;
};

export type Settings = {
  executablePath: string;
  guilds: GuildSettings[];
  automodEnabled: boolean;
  monitoringEnabled: boolean;
};
