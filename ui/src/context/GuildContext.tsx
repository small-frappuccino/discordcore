/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useMemo, type ReactNode } from "react";
import { useParams } from "react-router-dom";
import { useDashboardSession } from "./DashboardSessionContext";
import type { AccessibleGuild, DashboardGuildAccessLevel } from "../api/domains/guilds";

interface GuildContextValue {
  guildId: string;
  currentGuild: AccessibleGuild | null;
  accessLevel: DashboardGuildAccessLevel | null;
  canRead: boolean;
  canWrite: boolean;
}

const GuildContext = createContext<GuildContextValue | null>(null);

export function GuildProvider({ children }: { children: ReactNode }) {
  const { guildId } = useParams<{ guildId: string }>();
  const { accessibleGuilds, manageableGuilds, authState } = useDashboardSession();

  const id = guildId ?? "";
  
  const currentGuild =
    accessibleGuilds.find((guild) => guild.id === id) ??
    manageableGuilds.find((guild) => guild.id === id) ??
    null;

  const accessLevel = currentGuild?.access_level ?? null;
  const canRead = authState === "signed_in" && currentGuild !== null;
  const canWrite = authState === "signed_in" && accessLevel === "write";

  const value = useMemo(
    () => ({
      guildId: id,
      currentGuild,
      accessLevel,
      canRead,
      canWrite,
    }),
    [id, currentGuild, accessLevel, canRead, canWrite]
  );

  return (
    <GuildContext.Provider value={value}>
      {children}
    </GuildContext.Provider>
  );
}

export function useCurrentGuild() {
  const context = useContext(GuildContext);
  if (!context) {
    throw new Error("useCurrentGuild must be used within a GuildProvider");
  }
  return context;
}
