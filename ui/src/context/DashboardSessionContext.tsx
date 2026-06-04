/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useContext,
  useEffect,
  useEffectEvent,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  ControlApiClient,
  type AccessibleGuild,
  type AuthSessionResponse,
} from "../api/control";
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

        const guildsResponse = await activeClient.listAccessibleGuilds({
          fresh: freshGuilds,
        });
        const manageableGuildsResponse = await activeClient.listManageableGuilds();
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

  async function refreshSession() {
    await startSessionRefresh(client, {
      freshGuilds: true,
    });
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
        client,
        currentOriginLabel,
        manageableGuilds,
        notice,
        session,
        sessionAvatarURL,
        sessionLoading,
        applyBaseUrl,
        beginLogin,
        clearNotice: () => setNotice(null),
        logout,
        refreshSession,
        setBaseUrlDraft,
      }}
    >
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
