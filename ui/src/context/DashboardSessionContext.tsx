/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useContext,
  useEffect,
  useEffectEvent,
  useMemo,
  useRef,
  useState,
  useCallback,
  type ReactNode,
} from "react";
import { ControlApiClient } from "../api/client";
import type { AccessibleGuild } from "../api/domains/guilds";
import type { AuthSessionResponse } from "../api/domains/auth";
import { listAccessibleGuilds, listManageableGuilds, getGuildSettings, getBotProfiles, type BotProfile } from "../api/domains/guilds";
import { appRoutes } from "../app/routes";
import type { DashboardAuthState, Notice } from "../app/types";
import {
  buildUserAvatarURL,
  formatError,
  isValidBaseUrl,
  normalizeBaseUrlInput,
} from "../app/utils";

const defaultBaseUrl =
  import.meta.env.VITE_CONTROL_API_BASE_URL ?? window.location.origin;
const sessionRevalidationThrottleMs = 15_000;

interface DashboardSessionContextValue {
  authState: DashboardAuthState;
  baseUrl: string;
  baseUrlDraft: string;
  baseUrlDirty: boolean;
  busyLabel: string;
  accessibleGuilds: AccessibleGuild[];
  client: ControlApiClient;
  currentOriginLabel: string;
  manageableGuilds: AccessibleGuild[];
  notice: Notice | null;
  session: AuthSessionResponse | null;
  sessionAvatarURL: string | null;
  sessionLoading: boolean;
  applyBaseUrl: () => void;
  beginLogin: (nextPath?: string) => Promise<void>;
  clearNotice: () => void;
  logout: () => Promise<void>;
  refreshSession: () => Promise<void>;
  setBaseUrlDraft: (value: string) => void;
  mainBotProfile: BotProfile | null;
  fetchMainBotProfile: (guildId: string) => Promise<void>;
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
  const [accessibleGuilds, setAccessibleGuilds] = useState<AccessibleGuild[]>(
    [],
  );
  const [manageableGuilds, setManageableGuilds] = useState<AccessibleGuild[]>(
    [],
  );
  const [notice, setNotice] = useState<Notice | null>(null);
  const [sessionLoading, setSessionLoading] = useState(false);
  const [busyLabel, setBusyLabel] = useState("");
  const [mainBotProfile, setMainBotProfile] = useState<BotProfile | null>(null);
  const lastFreshSessionRefreshAtRef = useRef(0);
  const freshSessionRefreshRef = useRef<Promise<void> | null>(null);

  const client = useMemo(
    () =>
      new ControlApiClient({
        baseUrl,
      }),
    [baseUrl],
  );

  const currentOriginLabel = baseUrl.trim() === "" ? "Same origin" : baseUrl;
  const baseUrlDirty =
    normalizeBaseUrlInput(baseUrlDraft) !== normalizeBaseUrlInput(baseUrl);
  const sessionAvatarURL = session ? buildUserAvatarURL(session.user) : null;

  function clearSessionState() {
    setSession(null);
    setAccessibleGuilds([]);
    setManageableGuilds([]);
  }

  const performSessionRefresh = useEffectEvent(
    async (
      activeClient: ControlApiClient,
      options: {
        freshGuilds?: boolean;
      } = {},
    ) => {
      const freshGuilds = options.freshGuilds ?? false;
      setSessionLoading(true);

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

        const guildsResponse = await listAccessibleGuilds(activeClient, {
          fresh: freshGuilds,
        });
        const manageableGuildsResponse = await listManageableGuilds(activeClient);
        setAuthState("signed_in");
        setSession(probe.session);
        setAccessibleGuilds(guildsResponse.guilds);
        setManageableGuilds(manageableGuildsResponse.guilds);
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

  const startSessionRefresh = useEffectEvent(
    (
      activeClient: ControlApiClient,
      options: {
        freshGuilds?: boolean;
      } = {},
    ) => {
      const freshGuilds = options.freshGuilds ?? false;
      if (freshGuilds) {
        lastFreshSessionRefreshAtRef.current = Date.now();
      }

      const refreshPromise = performSessionRefresh(
        activeClient,
        options,
      ).finally(() => {
        if (freshSessionRefreshRef.current === refreshPromise) {
          freshSessionRefreshRef.current = null;
        }
      });

      if (freshGuilds) {
        freshSessionRefreshRef.current = refreshPromise;
      }

      return refreshPromise;
    },
  );

  const triggerFreshSessionRefresh = useEffectEvent(() => {
    if (authState === "checking" || authState === "oauth_unavailable") {
      return;
    }
    if (
      typeof document !== "undefined" &&
      document.visibilityState === "hidden"
    ) {
      return;
    }
    if (freshSessionRefreshRef.current !== null) {
      return;
    }
    if (
      Date.now() - lastFreshSessionRefreshAtRef.current <
      sessionRevalidationThrottleMs
    ) {
      return;
    }

    void startSessionRefresh(client, {
      freshGuilds: true,
    });
  });

  const startSessionRefreshRef = useRef(startSessionRefresh);
  const triggerFreshSessionRefreshRef = useRef(triggerFreshSessionRefresh);

  useEffect(() => {
    startSessionRefreshRef.current = startSessionRefresh;
  }, [startSessionRefresh]);

  useEffect(() => {
    triggerFreshSessionRefreshRef.current = triggerFreshSessionRefresh;
  }, [triggerFreshSessionRefresh]);

  useEffect(() => {
    void startSessionRefreshRef.current(client, {
      freshGuilds: true,
    });
  }, [client]);

  useEffect(() => {
    function handleFocus() {
      triggerFreshSessionRefreshRef.current();
    }

    function handleVisibilityChange() {
      if (document.visibilityState !== "visible") {
        return;
      }
      triggerFreshSessionRefreshRef.current();
    }

    window.addEventListener("focus", handleFocus);
    document.addEventListener("visibilitychange", handleVisibilityChange);
    return () => {
      window.removeEventListener("focus", handleFocus);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, []);

  // Removed selected guild sync effects

  const refreshSession = useCallback(async () => {
    await startSessionRefresh(client, {
      freshGuilds: true,
    });
  }, [startSessionRefresh, client]);

  const fetchMainBotProfile = useCallback(async (guildId: string) => {
    if (!guildId) {
      setMainBotProfile(null);
      return;
    }
    try {
      const [settings, profiles] = await Promise.all([
        getGuildSettings(client, guildId).catch(() => null),
        getBotProfiles(client, guildId).catch(() => []),
      ]);
      const mainId = settings?.workspace?.sections?.main_bot_instance_id;
      const main = profiles.find(p => p.logical_key === mainId) || profiles[0] || null;
      setMainBotProfile(main);
    } catch {
      setMainBotProfile(null);
    }
  }, [client]);

  const beginLogin = useCallback(async (nextPath: string = `${appRoutes.manage}/`) => {
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
  }, [client]);

  const logout = useCallback(async () => {
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
  }, [client]);

  const applyBaseUrl = useCallback(() => {
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
  }, [baseUrlDraft]);

  const clearNotice = useCallback(() => setNotice(null), []);

  const contextValue = useMemo(
    () => ({
      authState,
      baseUrl,
      baseUrlDraft,
      baseUrlDirty,
      accessibleGuilds,
      busyLabel,
      client,
      currentOriginLabel,
      manageableGuilds,
      notice,
      session,
      sessionAvatarURL,
      sessionLoading,
      applyBaseUrl,
      beginLogin,
      clearNotice,
      logout,
      refreshSession,
      setBaseUrlDraft,
      mainBotProfile,
      fetchMainBotProfile,
    }),
    [
      authState,
      baseUrl,
      baseUrlDraft,
      baseUrlDirty,
      accessibleGuilds,
      busyLabel,
      client,
      currentOriginLabel,
      manageableGuilds,
      notice,
      session,
      sessionAvatarURL,
      sessionLoading,
      applyBaseUrl,
      beginLogin,
      clearNotice,
      logout,
      refreshSession,
      setBaseUrlDraft,
      mainBotProfile,
      fetchMainBotProfile,
    ]
  );

  return (
    <DashboardSessionContext.Provider value={contextValue}>
      {children}
    </DashboardSessionContext.Provider>
  );
}

export function useDashboardSession() {
  const context = useContext(DashboardSessionContext);
  if (context === null) {
    throw new Error(
      "useDashboardSession must be used inside DashboardSessionProvider",
    );
  }
  return context;
}
