/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import {
  ControlApiClient,
  type AuthSessionResponse,
  type ManageableGuild,
} from "../api/control";
import { appRoutes } from "../app/routes";
import type { DashboardAuthState, Notice } from "../app/types";
import {
  buildGuildIconURL,
  buildUserAvatarURL,
  formatError,
  isValidBaseUrl,
  normalizeBaseUrlInput,
  resolveGuildSelection,
} from "../app/utils";
import {
  prefetchGuildDashboardResources,
  resetGuildResourceCache,
} from "../features/features/guildResourceCache";

const defaultBaseUrl =
  import.meta.env.VITE_CONTROL_API_BASE_URL ?? window.location.origin;
const preferredGuildID = import.meta.env.VITE_CONTROL_API_GUILD_ID ?? "";

interface DashboardSessionContextValue {
  authState: DashboardAuthState;
  baseUrl: string;
  baseUrlDraft: string;
  baseUrlDirty: boolean;
  busyLabel: string;
  canManageGuild: boolean;
  client: ControlApiClient;
  currentOriginLabel: string;
  manageableGuilds: ManageableGuild[];
  notice: Notice | null;
  selectedGuild: ManageableGuild | null;
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
  const [baseUrl, setBaseUrl] = useState(defaultBaseUrl);
  const [baseUrlDraft, setBaseUrlDraft] = useState(defaultBaseUrl);
  const [authState, setAuthState] = useState<DashboardAuthState>("checking");
  const [session, setSession] = useState<AuthSessionResponse | null>(null);
  const [manageableGuilds, setManageableGuilds] = useState<ManageableGuild[]>([]);
  const [selectedGuildID, setSelectedGuildID] = useState(preferredGuildID);
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
    manageableGuilds.find((guild) => guild.id === selectedGuildID) ?? null;
  const currentOriginLabel = baseUrl.trim() === "" ? "Same origin" : baseUrl;
  const baseUrlDirty =
    normalizeBaseUrlInput(baseUrlDraft) !== normalizeBaseUrlInput(baseUrl);
  const canManageGuild =
    authState === "signed_in" && selectedGuildID.trim() !== "";
  const sessionAvatarURL = session ? buildUserAvatarURL(session.user) : null;
  const selectedGuildIconURL = selectedGuild
    ? buildGuildIconURL(selectedGuild)
    : null;

  function clearSessionState() {
    setSession(null);
    setManageableGuilds([]);
    setSelectedGuildID("");
    resetGuildResourceCache();
  }

  async function performSessionRefresh(activeClient: ControlApiClient) {
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

      setAuthState("signed_in");
      setSession(probe.session);

      const guildsResponse = await activeClient.listManageableGuilds();
      const nextGuildID = resolveGuildSelection(
        selectedGuildID,
        preferredGuildID,
        guildsResponse.guilds,
      );
      setManageableGuilds(guildsResponse.guilds);
      setSelectedGuildID(nextGuildID);
      setNotice(null);
      if (nextGuildID !== "") {
        void prefetchGuildDashboardResources(activeClient, baseUrl, nextGuildID).catch(
          () => {},
        );
      }
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
  }

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

  async function refreshSession() {
    await performSessionRefresh(client);
  }

  async function beginLogin(nextPath: string = appRoutes.dashboardOverview) {
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
        busyLabel,
        canManageGuild,
        client,
        currentOriginLabel,
        manageableGuilds,
        notice,
        selectedGuild,
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
