/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useContext,
  useEffect,
  useEffectEvent,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { useLocation } from "react-router-dom";
import {
  ControlApiClient,
  type AccessibleGuild,
  type AuthSessionResponse,
  type DashboardGuildAccessLevel,
} from "../api/control";
import { appRoutes } from "../app/routes";
import type { DashboardAuthState, Notice } from "../app/types";
import {
  buildGuildIconURL,
  buildUserAvatarURL,
  formatError,
  isValidBaseUrl,
  normalizeBaseUrlInput,
} from "../app/utils";
import {
  prefetchGuildDashboardResources,
  resetGuildResourceCache,
} from "../features/features/guildResourceCache";

const defaultBaseUrl =
  import.meta.env.VITE_CONTROL_API_BASE_URL ?? window.location.origin;

interface DashboardSessionContextValue {
  authState: DashboardAuthState;
  baseUrl: string;
  baseUrlDraft: string;
  baseUrlDirty: boolean;
  busyLabel: string;
  accessibleGuilds: AccessibleGuild[];
  canEditSelectedGuild: boolean;
  canReadSelectedGuild: boolean;
  canManageGuild: boolean;
  client: ControlApiClient;
  currentOriginLabel: string;
  manageableGuilds: AccessibleGuild[];
  notice: Notice | null;
  selectedGuild: AccessibleGuild | null;
  selectedGuildAccessLevel: DashboardGuildAccessLevel | null;
  selectedGuildIconURL: string | null;
  selectedGuildID: string;
  session: AuthSessionResponse | null;
  sessionAvatarURL: string | null;
  sessionLoading: boolean;
  applyBaseUrl: () => void;
  beginLogin: (nextPath?: string) => Promise<void>;
  clearNotice: () => void;
  logout: () => Promise<void>;
  refreshSession: () => Promise<void>;
  setBaseUrlDraft: (value: string) => void;
  setSelectedGuildID: (value: string) => void;
}

const DashboardSessionContext =
  createContext<DashboardSessionContextValue | null>(null);

export function DashboardSessionProvider({
  children,
}: {
  children: ReactNode;
}) {
  const location = useLocation();
  const [baseUrl, setBaseUrl] = useState(defaultBaseUrl);
  const [baseUrlDraft, setBaseUrlDraft] = useState(defaultBaseUrl);
  const [authState, setAuthState] = useState<DashboardAuthState>("checking");
  const [session, setSession] = useState<AuthSessionResponse | null>(null);
  const [accessibleGuilds, setAccessibleGuilds] = useState<AccessibleGuild[]>([]);
  const [manageableGuilds, setManageableGuilds] = useState<AccessibleGuild[]>([]);
  const [selectedGuildID, setSelectedGuildID] = useState("");
  const [notice, setNotice] = useState<Notice | null>(null);
  const [sessionLoading, setSessionLoading] = useState(false);
  const [busyLabel, setBusyLabel] = useState("");

  const client = useMemo(
    () =>
      new ControlApiClient({
        baseUrl,
      }),
    [baseUrl],
  );

  const selectedGuild =
    accessibleGuilds.find((guild) => guild.id === selectedGuildID) ??
    manageableGuilds.find((guild) => guild.id === selectedGuildID) ??
    null;
  const selectedGuildAccessLevel = selectedGuild?.access_level ?? null;
  const currentOriginLabel = baseUrl.trim() === "" ? "Same origin" : baseUrl;
  const baseUrlDirty =
    normalizeBaseUrlInput(baseUrlDraft) !== normalizeBaseUrlInput(baseUrl);
  const canReadSelectedGuild =
    authState === "signed_in" && selectedGuild !== null;
  const canEditSelectedGuild = selectedGuildAccessLevel === "write";
  const canManageGuild = canEditSelectedGuild;
  const sessionAvatarURL = session ? buildUserAvatarURL(session.user) : null;
  const selectedGuildIconURL = selectedGuild
    ? buildGuildIconURL(selectedGuild)
    : null;

  function clearSessionState() {
    setSession(null);
    setAccessibleGuilds([]);
    setManageableGuilds([]);
    setSelectedGuildID("");
    resetGuildResourceCache();
  }

  const performSessionRefresh = useEffectEvent(
    async (activeClient: ControlApiClient) => {
      setSessionLoading(true);
      setBusyLabel("Refreshing session");

      try {
        const probe = await activeClient.getSessionStatus();
        if (probe.status === "oauth_unavailable") {
          setAuthState("oauth_unavailable");
          clearSessionState();
          setNotice({
            tone: "info",
            message: "Discord OAuth is unavailable on this control server.",
          });
          return;
        }

        if (probe.status === "unauthorized") {
          setAuthState("signed_out");
          clearSessionState();
          setNotice({
            tone: "info",
            message: "Sign in with Discord to continue.",
          });
          return;
        }

        const [guildsResponse, manageableGuildsResponse] = await Promise.all([
          activeClient.listAccessibleGuilds(),
          activeClient.listManageableGuilds(),
        ]);
        const availableGuildIDs = new Set([
          ...guildsResponse.guilds.map((guild) => guild.id),
          ...manageableGuildsResponse.guilds.map((guild) => guild.id),
        ]);
        const routedGuildID = getRoutedGuildID(window.location.pathname);
        const preferredGuildID =
          selectedGuildID.trim() !== "" ? selectedGuildID.trim() : routedGuildID;
        const nextGuildID =
          preferredGuildID !== "" && availableGuildIDs.has(preferredGuildID)
            ? preferredGuildID
            : "";
        if (nextGuildID !== "") {
          await prefetchGuildDashboardResources(activeClient, baseUrl, nextGuildID);
        }
        setAuthState("signed_in");
        setSession(probe.session);
        setAccessibleGuilds(guildsResponse.guilds);
        setManageableGuilds(manageableGuildsResponse.guilds);
        setSelectedGuildID(nextGuildID);
        setNotice(null);
      } catch (error) {
        setAuthState("signed_out");
        clearSessionState();
        setNotice({
          tone: "error",
          message: formatError(error),
        });
      } finally {
        setSessionLoading(false);
        setBusyLabel("");
      }
    },
  );

  useEffect(() => {
    void performSessionRefresh(client);
  }, [client]);

  useEffect(() => {
    if (authState !== "signed_in" || selectedGuildID.trim() === "") {
      return;
    }
    void prefetchGuildDashboardResources(client, baseUrl, selectedGuildID.trim()).catch(
      () => {},
    );
  }, [authState, baseUrl, client, selectedGuildID]);

  useEffect(() => {
    if (authState !== "signed_in") {
      return;
    }

    const routedGuildID = getRoutedGuildID(location.pathname);
    if (routedGuildID === "" || routedGuildID === selectedGuildID.trim()) {
      return;
    }

    const availableGuildIDs = new Set([
      ...accessibleGuilds.map((guild) => guild.id),
      ...manageableGuilds.map((guild) => guild.id),
    ]);
    if (!availableGuildIDs.has(routedGuildID)) {
      return;
    }

    setSelectedGuildID(routedGuildID);
  }, [
    accessibleGuilds,
    authState,
    location.pathname,
    manageableGuilds,
    selectedGuildID,
  ]);

  async function refreshSession() {
    await performSessionRefresh(client);
  }

  async function beginLogin(nextPath: string = `${appRoutes.manage}/`) {
    try {
      const oauthStatus = await client.getDiscordOAuthStatus(nextPath);
      const loginURL = oauthStatus.login_url?.trim() ?? "";
      if (!oauthStatus.oauth_configured || loginURL === "") {
        setNotice({
          tone: "info",
          message: "Discord OAuth is unavailable on this control server.",
        });
        return;
      }
      window.location.assign(loginURL);
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    }
  }

  async function logout() {
    setSessionLoading(true);
    setBusyLabel("Signing out");

    try {
      await client.logout();
      setAuthState("signed_out");
      clearSessionState();
      setNotice({
        tone: "info",
        message: "Signed out.",
      });
    } catch (error) {
      setNotice({
        tone: "error",
        message: formatError(error),
      });
    } finally {
      setSessionLoading(false);
      setBusyLabel("");
    }
  }

  function applyBaseUrl() {
    const normalized = normalizeBaseUrlInput(baseUrlDraft);
    if (!isValidBaseUrl(normalized)) {
      setNotice({
        tone: "error",
        message: "Enter a valid control connection URL.",
      });
      return;
    }

    setAuthState("checking");
    setBaseUrl(normalized);
    setBaseUrlDraft(normalized);
    setNotice({
      tone: "info",
      message:
        normalized === ""
          ? "Using the current origin for dashboard requests."
          : `Using ${normalized} for dashboard requests.`,
    });
  }

  return (
    <DashboardSessionContext.Provider
      value={{
        authState,
        baseUrl,
        baseUrlDraft,
        baseUrlDirty,
        accessibleGuilds,
        busyLabel,
        canEditSelectedGuild,
        canReadSelectedGuild,
        canManageGuild,
        client,
        currentOriginLabel,
        manageableGuilds,
        notice,
        selectedGuild,
        selectedGuildAccessLevel,
        selectedGuildIconURL,
        selectedGuildID,
        session,
        sessionAvatarURL,
        sessionLoading,
        applyBaseUrl,
        beginLogin,
        clearNotice: () => setNotice(null),
        logout,
        refreshSession,
        setBaseUrlDraft,
        setSelectedGuildID,
      }}
    >
      {children}
    </DashboardSessionContext.Provider>
  );
}

export function useDashboardSession() {
  const context = useContext(DashboardSessionContext);
  if (context === null) {
    throw new Error("useDashboardSession must be used inside DashboardSessionProvider");
  }
  return context;
}

function getRoutedGuildID(pathname: string) {
  const match = pathname.match(/^\/manage\/([^/]+)/);
  return decodeURIComponent(match?.[1] ?? "").trim();
}
