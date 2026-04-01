import type {
  AccessibleGuild,
  AuthSessionResponse,
  DiscordOAuthUser,
} from "../api/control";
import type { DashboardAuthState } from "./types";

export function formatError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

export function formatSessionTitle(session: AuthSessionResponse): string {
  return (
    session.user.global_name?.trim() ||
    session.user.username.trim() ||
    session.user.id
  );
}

export function formatUserLabel(session: AuthSessionResponse): string {
  return `${formatSessionTitle(session)} (${session.user.id})`;
}

export function formatAuthStateLabel(authState: DashboardAuthState): string {
  switch (authState) {
    case "checking":
      return "Checking access";
    case "signed_out":
      return "Signed out";
    case "oauth_unavailable":
      return "OAuth unavailable";
    case "signed_in":
      return "Signed in";
    default:
      return "Unknown";
  }
}

export function formatAuthSupportText(
  authState: DashboardAuthState,
  accessibleGuildCount: number,
): string {
  switch (authState) {
    case "checking":
      return "Checking your Discord session.";
    case "signed_out":
      return "Sign in with Discord to access your dashboard servers.";
    case "oauth_unavailable":
      return "Discord OAuth is not configured on this control server.";
    case "signed_in":
      return `${accessibleGuildCount} server${accessibleGuildCount === 1 ? "" : "s"} available.`;
    default:
      return "Session state unavailable.";
  }
}

export function formatTimestamp(
  value: number | null,
  fallback = "Not yet",
): string {
  if (value === null) {
    return fallback;
  }

  return new Intl.DateTimeFormat(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

export function resolveGuildSelection(
  currentGuildID: string,
  preferredGuildID: string,
  guilds: AccessibleGuild[],
): string {
  const availableGuildIDs = new Set(guilds.map((guild) => guild.id));
  if (currentGuildID.trim() !== "" && availableGuildIDs.has(currentGuildID.trim())) {
    return currentGuildID.trim();
  }
  if (
    preferredGuildID.trim() !== "" &&
    availableGuildIDs.has(preferredGuildID.trim())
  ) {
    return preferredGuildID.trim();
  }
  if (guilds.length > 0) {
    return guilds[0].id;
  }
  return "";
}

export function normalizeBaseUrlInput(raw: string): string {
  return raw.trim().replace(/\/+$/, "");
}

export function isValidBaseUrl(raw: string): boolean {
  if (raw === "") {
    return true;
  }

  try {
    const parsed = new URL(raw);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

export function buildGuildIconURL(guild: AccessibleGuild): string | null {
  if (guild.bot_present === false || !guild.icon) {
    return null;
  }
  return `https://cdn.discordapp.com/icons/${guild.id}/${guild.icon}.webp?size=128`;
}

export function buildUserAvatarURL(user: DiscordOAuthUser): string | null {
  if (!user.avatar) {
    return null;
  }
  return `https://cdn.discordapp.com/avatars/${user.id}/${user.avatar}.webp?size=128`;
}

export function getInitials(value: string): string {
  const parts = value
    .trim()
    .split(/\s+/)
    .filter(Boolean)
    .slice(0, 2);
  if (parts.length === 0) {
    return "?";
  }
  return parts.map((part) => part[0]?.toUpperCase() ?? "").join("");
}
