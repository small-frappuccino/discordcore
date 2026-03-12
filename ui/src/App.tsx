import {
  startTransition,
  useCallback,
  useDeferredValue,
  useEffect,
  useMemo,
  useState,
} from "react";
import {
  type AuthSessionResponse,
  type DiscordOAuthUser,
  type EmbedUpdateTargetConfig,
  type ManageableGuild,
  type PartnerBoardConfig,
  type PartnerBoardTemplateConfig,
  type PartnerEntryConfig,
  ControlApiClient,
} from "./api/control";

type StatusKind = "idle" | "success" | "error" | "info";
type DashboardAuthState =
  | "checking"
  | "signed_out"
  | "signed_in"
  | "oauth_unavailable";
type AppView = "landing" | "control-panel";

interface StatusState {
  kind: StatusKind;
  message: string;
}

interface TargetFormState {
  type: "webhook_message" | "channel_message";
  messageID: string;
  webhookURL: string;
  channelID: string;
}

interface TemplateFormState {
  title: string;
  intro: string;
  sectionHeaderTemplate: string;
  lineTemplate: string;
  emptyStateText: string;
}

interface PartnerFormState {
  fandom: string;
  name: string;
  link: string;
}

interface PartnerUpdateFormState {
  currentName: string;
  fandom: string;
  name: string;
  link: string;
}

interface WorkflowStep {
  id: string;
  title: string;
  description: string;
  completed: boolean;
  current: boolean;
  sectionId: string;
}

interface FandomHighlight {
  label: string;
  count: number;
}

const initialTargetForm: TargetFormState = {
  type: "channel_message",
  messageID: "",
  webhookURL: "",
  channelID: "",
};

const initialTemplateForm: TemplateFormState = {
  title: "",
  intro: "",
  sectionHeaderTemplate: "",
  lineTemplate: "",
  emptyStateText: "",
};

const initialPartnerForm: PartnerFormState = {
  fandom: "",
  name: "",
  link: "",
};

const initialPartnerUpdateForm: PartnerUpdateFormState = {
  currentName: "",
  fandom: "",
  name: "",
  link: "",
};

const defaultBaseUrl =
  import.meta.env.VITE_CONTROL_API_BASE_URL ?? window.location.origin;
const preferredGuildID = import.meta.env.VITE_CONTROL_API_GUILD_ID ?? "";
const lockedTheme = {
  id: "bnnuy-ledger",
  label: "Bnnuy Ledger",
  helper: "Warm oat, blush, and moss accents tuned for a softer control surface",
} as const;
const siteBrandIconSrc = `${import.meta.env.BASE_URL}brand/alicebot.webp`;
const dashboardHomePath = "/dashboard/";
const controlPanelPath = "/dashboard/control-panel";
const landingNavLinks = [
  { id: "about", label: "About" },
  { id: "features", label: "Features" },
  { id: "status", label: "Status" },
  { id: "invite", label: "Add to server" },
  { id: "documentation", label: "Documentation" },
  { id: "support", label: "Support" },
  { id: "premium", label: "Premium" },
] as const;
const landingFeatureCards = [
  {
    title: "Clear moderation flows",
    body: "Keep the public entry calm and readable while operational controls stay behind the authenticated workspace.",
  },
  {
    title: "Discord-authenticated access",
    body: "Guild editing remains gated by Discord OAuth so the landing page never mixes product marketing with configuration actions.",
  },
  {
    title: "Responsive by default",
    body: "The shell is built as a scalable front-end surface with reusable sections, accessible contrast, and mobile-safe spacing.",
  },
] as const;
const landingTrustPoints = [
  "Public landing page separated from guild management",
  "Dashboard stays mounted under `/dashboard/`",
  "Discord OAuth is the only path into configuration",
] as const;
const sectionLinks = [
  { id: "overview", label: "Overview" },
  { id: "delivery", label: "Delivery" },
  { id: "template", label: "Template" },
  { id: "partners", label: "Partners" },
] as const;
const templateFieldCount = 5;

export default function App() {
  const [baseUrl, setBaseUrl] = useState(defaultBaseUrl);
  const [baseUrlInput, setBaseUrlInput] = useState(defaultBaseUrl);
  const [guildID, setGuildID] = useState(preferredGuildID);
  const [board, setBoard] = useState<PartnerBoardConfig | null>(null);
  const [targetForm, setTargetForm] = useState(initialTargetForm);
  const [templateForm, setTemplateForm] = useState(initialTemplateForm);
  const [partnerForm, setPartnerForm] = useState(initialPartnerForm);
  const [partnerUpdateForm, setPartnerUpdateForm] = useState(
    initialPartnerUpdateForm,
  );
  const [partnerDeleteName, setPartnerDeleteName] = useState("");
  const [partnerSearch, setPartnerSearch] = useState("");
  const [selectedPartnerName, setSelectedPartnerName] = useState("");
  const [lastLoadedAt, setLastLoadedAt] = useState<number | null>(null);
  const [busyAction, setBusyAction] = useState("");
  const [status, setStatus] = useState<StatusState>({
    kind: "idle",
    message: "Ready",
  });
  const [loading, setLoading] = useState(false);
  const [authState, setAuthState] = useState<DashboardAuthState>("checking");
  const [session, setSession] = useState<AuthSessionResponse | null>(null);
  const [manageableGuilds, setManageableGuilds] = useState<ManageableGuild[]>(
    [],
  );
  const [appPath, setAppPath] = useState(() =>
    normalizeDashboardAppPath(window.location.pathname),
  );

  const client = useMemo(
    () =>
      new ControlApiClient({
        baseUrl,
      }),
    [baseUrl],
  );

  const deferredPartnerSearch = useDeferredValue(
    partnerSearch.trim().toLowerCase(),
  );
  const partners = useMemo(() => board?.partners ?? [], [board?.partners]);
  const filteredPartners = useMemo(() => {
    if (deferredPartnerSearch === "") {
      return partners;
    }

    return partners.filter((partner) => {
      const haystack = [
        partner.fandom ?? "",
        partner.name,
        partner.link,
      ].join(" ");
      return haystack.toLowerCase().includes(deferredPartnerSearch);
    });
  }, [deferredPartnerSearch, partners]);
  const selectedGuild = useMemo(
    () => manageableGuilds.find((guild) => guild.id === guildID) ?? null,
    [guildID, manageableGuilds],
  );
  const targetDraft = useMemo(() => buildTargetPayload(targetForm), [targetForm]);
  const templateDraft = useMemo(
    () => buildTemplateDraft(templateForm, board?.template),
    [board?.template, templateForm],
  );
  const targetConfigured = useMemo(
    () => isTargetConfigured(targetDraft),
    [targetDraft],
  );
  const templateConfigured = useMemo(
    () => isTemplateConfigured(templateDraft),
    [templateDraft],
  );
  const templateCompletion = useMemo(
    () => countFilledTemplateFields(templateDraft),
    [templateDraft],
  );
  const fandomHighlights = useMemo(
    () => collectFandomHighlights(partners),
    [partners],
  );
  const workflowSteps = useMemo(
    () =>
      buildWorkflowSteps(
        authState,
        guildID,
        board !== null,
        targetConfigured,
        templateConfigured,
        partners.length,
      ),
    [
      authState,
      board,
      guildID,
      partners.length,
      targetConfigured,
      templateConfigured,
    ],
  );
  const completedSteps = workflowSteps.filter((step) => step.completed).length;
  const readinessScore = Math.round(
    (completedSteps / workflowSteps.length) * 100,
  );
  const nextStep = workflowSteps.find((step) => !step.completed) ?? null;
  const canManageGuild =
    authState === "signed_in" && guildID.trim() !== "" && !loading;
  const baseUrlDirty =
    normalizeBaseUrlInput(baseUrlInput) !== normalizeBaseUrlInput(baseUrl);
  const activeOriginLabel = baseUrl.trim() === "" ? "Same origin" : baseUrl;
  const sessionAvatarURL = session ? buildUserAvatarURL(session.user) : null;
  const selectedGuildIcon = selectedGuild ? buildGuildIconURL(selectedGuild) : null;
  const filteredPartnerLabel =
    deferredPartnerSearch === ""
      ? `${partners.length} partner${partners.length === 1 ? "" : "s"}`
      : `${filteredPartners.length} of ${partners.length} partners`;
  const appView = useMemo<AppView>(() => resolveAppView(appPath), [appPath]);
  const signedInUserName = session !== null ? formatSessionTitle(session) : null;

  const withBusyState = useCallback(
    async (label: string, operation: () => Promise<void>) => {
      setBusyAction(label);
      setLoading(true);
      try {
        await operation();
      } finally {
        setLoading(false);
        setBusyAction("");
      }
    },
    [],
  );

  const clearWorkspaceDrafts = useCallback(() => {
    setBoard(null);
    setTargetForm(initialTargetForm);
    setTemplateForm(initialTemplateForm);
    setPartnerUpdateForm(initialPartnerUpdateForm);
    setPartnerDeleteName("");
    setPartnerSearch("");
    setSelectedPartnerName("");
    setLastLoadedAt(null);
  }, []);

  const resetLoadedBoard = useCallback(() => {
    setGuildID("");
    setPartnerForm(initialPartnerForm);
    clearWorkspaceDrafts();
  }, [clearWorkspaceDrafts]);

  const refreshSession = useCallback(async () => {
    await withBusyState("Refreshing dashboard session", async () => {
      try {
        const probe = await client.getSessionStatus();
        if (probe.status === "oauth_unavailable") {
          setAuthState("oauth_unavailable");
          setSession(null);
          setManageableGuilds([]);
          resetLoadedBoard();
          setStatus({
            kind: "info",
            message: "OAuth unavailable.",
          });
          return;
        }

        if (probe.status === "unauthorized") {
          setAuthState("signed_out");
          setSession(null);
          setManageableGuilds([]);
          resetLoadedBoard();
          setStatus({
            kind: "info",
            message: "Sign in to continue.",
          });
          return;
        }

        setAuthState("signed_in");
        setSession(probe.session);

        const guildsResponse = await client.listManageableGuilds();
        setManageableGuilds(guildsResponse.guilds);
        setGuildID((current: string) =>
          resolveGuildSelection(current, preferredGuildID, guildsResponse.guilds),
        );

        if (guildsResponse.guilds.length === 0) {
          resetLoadedBoard();
          setStatus({
            kind: "info",
            message: "No manageable guilds.",
          });
          return;
        }

        setStatus({
          kind: "success",
          message: `Signed in: ${formatUserLabel(probe.session)}.`,
        });
      } catch (error) {
        setAuthState("signed_out");
        setSession(null);
        setManageableGuilds([]);
        resetLoadedBoard();
        setStatus({
          kind: "error",
          message: formatError(error),
        });
      }
    });
  }, [client, resetLoadedBoard, withBusyState]);

  useEffect(() => {
    void refreshSession();
  }, [refreshSession]);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", lockedTheme.id);
  }, []);

  useEffect(() => {
    const syncAppPath = () => {
      setAppPath(normalizeDashboardAppPath(window.location.pathname));
    };

    window.addEventListener("popstate", syncAppPath);
    return () => {
      window.removeEventListener("popstate", syncAppPath);
    };
  }, []);

  async function logout() {
    await withBusyState("Signing out", async () => {
      try {
        await client.logout();
        setAuthState("signed_out");
        setSession(null);
        setManageableGuilds([]);
        resetLoadedBoard();
        setStatus({
          kind: "info",
          message: "Signed out.",
        });
        navigateToAppPath(dashboardHomePath);
      } catch (error) {
        setStatus({
          kind: "error",
          message: formatError(error),
        });
      }
    });
  }

  async function refreshBoard() {
    if (!ensureGuildSelected()) {
      return;
    }

    const trimmedGuild = guildID.trim();
    await withBusyState("Loading partner board", async () => {
      try {
        await loadBoardData(trimmedGuild);
        setStatus({
          kind: "success",
          message: "Board loaded.",
        });
      } catch (error) {
        setStatus({
          kind: "error",
          message: formatError(error),
        });
      }
    });
  }

  async function refreshPartnersOnly() {
    if (!ensureGuildSelected()) {
      return;
    }

    const trimmedGuild = guildID.trim();
    await withBusyState("Refreshing partners", async () => {
      try {
        const count = await loadPartnersData(trimmedGuild);
        setStatus({
          kind: "success",
          message: `${count} partners loaded.`,
        });
      } catch (error) {
        setStatus({
          kind: "error",
          message: formatError(error),
        });
      }
    });
  }

  async function saveTarget() {
    if (!ensureGuildSelected()) {
      return;
    }

    const validationError = validateTargetForm(targetForm);
    if (validationError !== null) {
      setStatus({
        kind: "error",
        message: validationError,
      });
      return;
    }

    const payload = buildTargetPayload(targetForm);
    const trimmedGuild = guildID.trim();
    await withBusyState("Saving delivery settings", async () => {
      try {
        await client.setPartnerBoardTarget(trimmedGuild, payload);
        await loadBoardData(trimmedGuild);
        setStatus({
          kind: "success",
          message: "Target saved.",
        });
      } catch (error) {
        setStatus({
          kind: "error",
          message: formatError(error),
        });
      }
    });
  }

  async function saveTemplate() {
    if (!ensureGuildSelected()) {
      return;
    }

    const payload = buildTemplateDraft(templateForm, board?.template);
    const trimmedGuild = guildID.trim();
    await withBusyState("Saving template settings", async () => {
      try {
        await client.setPartnerBoardTemplate(trimmedGuild, payload);
        await loadBoardData(trimmedGuild);
        setStatus({
          kind: "success",
          message: "Template saved.",
        });
      } catch (error) {
        setStatus({
          kind: "error",
          message: formatError(error),
        });
      }
    });
  }

  async function addPartner() {
    if (!ensureGuildSelected()) {
      return;
    }

    const validationError = validatePartnerForm(partnerForm);
    if (validationError !== null) {
      setStatus({
        kind: "error",
        message: validationError,
      });
      return;
    }

    const trimmedGuild = guildID.trim();
    await withBusyState("Adding partner", async () => {
      try {
        await client.createPartner(trimmedGuild, {
          fandom: partnerForm.fandom.trim(),
          name: partnerForm.name.trim(),
          link: partnerForm.link.trim(),
        });
        setPartnerForm(initialPartnerForm);
        await loadBoardData(trimmedGuild);
        setStatus({
          kind: "success",
          message: "Partner created.",
        });
      } catch (error) {
        setStatus({
          kind: "error",
          message: formatError(error),
        });
      }
    });
  }

  async function updatePartner() {
    if (!ensureGuildSelected()) {
      return;
    }

    const validationError = validatePartnerUpdateForm(partnerUpdateForm);
    if (validationError !== null) {
      setStatus({
        kind: "error",
        message: validationError,
      });
      return;
    }

    const trimmedGuild = guildID.trim();
    await withBusyState("Updating partner", async () => {
      try {
        await client.updatePartner(trimmedGuild, partnerUpdateForm.currentName.trim(), {
          fandom: partnerUpdateForm.fandom.trim(),
          name: partnerUpdateForm.name.trim(),
          link: partnerUpdateForm.link.trim(),
        });
        setPartnerUpdateForm(initialPartnerUpdateForm);
        setPartnerDeleteName("");
        setSelectedPartnerName("");
        await loadBoardData(trimmedGuild);
        setStatus({
          kind: "success",
          message: "Partner updated.",
        });
      } catch (error) {
        setStatus({
          kind: "error",
          message: formatError(error),
        });
      }
    });
  }

  async function deletePartner() {
    if (!ensureGuildSelected()) {
      return;
    }

    if (partnerDeleteName.trim() === "") {
      setStatus({
        kind: "error",
        message: "Partner name to delete is required.",
      });
      return;
    }

    const trimmedGuild = guildID.trim();
    await withBusyState("Deleting partner", async () => {
      try {
        await client.deletePartner(trimmedGuild, partnerDeleteName.trim());
        setPartnerDeleteName("");
        setSelectedPartnerName("");
        await loadBoardData(trimmedGuild);
        setStatus({
          kind: "success",
          message: "Partner deleted.",
        });
      } catch (error) {
        setStatus({
          kind: "error",
          message: formatError(error),
        });
      }
    });
  }

  async function syncBoard() {
    if (!ensureGuildSelected()) {
      return;
    }

    const trimmedGuild = guildID.trim();
    await withBusyState("Requesting board sync", async () => {
      try {
        await client.syncPartnerBoard(trimmedGuild);
        setStatus({
          kind: "success",
          message: "Sync requested.",
        });
      } catch (error) {
        setStatus({
          kind: "error",
          message: formatError(error),
        });
      }
    });
  }

  function navigateToAppPath(nextPath: string) {
    const normalized = normalizeDashboardAppPath(nextPath);
    if (normalized !== window.location.pathname) {
      window.history.pushState({}, "", normalized);
    }
    setAppPath(normalized);
    window.scrollTo({ top: 0, left: 0, behavior: "smooth" });
  }

  function openLandingSection(sectionId: string) {
    if (appView !== "landing") {
      navigateToAppPath(dashboardHomePath);
      window.requestAnimationFrame(() => {
        scrollToSection(sectionId);
      });
      return;
    }

    scrollToSection(sectionId);
  }

  async function beginLogin(nextPath = appPath) {
    try {
      const oauthStatus = await client.getDiscordOAuthStatus(nextPath);
      const loginURL = oauthStatus.login_url?.trim() ?? "";
      if (!oauthStatus.oauth_configured || loginURL === "") {
        setStatus({
          kind: "info",
          message: "OAuth unavailable.",
        });
        return;
      }
      window.location.assign(loginURL);
    } catch (error) {
      setStatus({
        kind: "error",
        message: formatError(error),
      });
    }
  }

  function applyBaseUrl() {
    const normalized = normalizeBaseUrlInput(baseUrlInput);
    if (!isValidBaseUrl(normalized)) {
      setStatus({
        kind: "error",
        message: "Enter a valid endpoint URL.",
      });
      return;
    }

    if (normalized === normalizeBaseUrlInput(baseUrl)) {
      setBaseUrlInput(normalized);
      return;
    }

    setAuthState("checking");
    setSession(null);
    setManageableGuilds([]);
    resetLoadedBoard();
    setBaseUrl(normalized);
    setBaseUrlInput(normalized);
    setStatus({
      kind: "info",
      message:
        normalized === "" ? "Using same origin." : `Endpoint: ${normalized}`,
    });
  }

  function scrollToSection(sectionId: string) {
    const section = document.getElementById(sectionId);
    if (section !== null) {
      section.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }

  function loadPartnerIntoEditor(partner: PartnerEntryConfig) {
    startTransition(() => {
      setSelectedPartnerName(partner.name);
      setPartnerUpdateForm({
        currentName: partner.name,
        fandom: partner.fandom ?? "",
        name: partner.name,
        link: partner.link,
      });
      setPartnerDeleteName(partner.name);
    });
    setStatus({
      kind: "info",
      message: `${partner.name} selected.`,
    });
    scrollToSection("partner-editor");
  }

  function handleGuildSelection(nextGuildID: string) {
    setGuildID(nextGuildID);
    clearWorkspaceDrafts();
  }

  function ensureGuildSelected(): boolean {
    if (authState !== "signed_in") {
      setStatus({
        kind: "error",
        message: "Sign in first.",
      });
      return false;
    }
    if (guildID.trim() === "") {
      setStatus({
        kind: "error",
        message: "Select a guild.",
      });
      return false;
    }
    return true;
  }


  async function loadBoardData(nextGuildID: string) {
    const response = await client.getPartnerBoard(nextGuildID);
    setBoard(response.partner_board);
    applyBoardToForms(response.partner_board);
    setSelectedPartnerName("");
    setLastLoadedAt(Date.now());
  }

  async function loadPartnersData(nextGuildID: string): Promise<number> {
    const response = await client.listPartners(nextGuildID);
    setBoard((prev) => ({
      ...(prev ?? {}),
      partners: response.partners,
    }));
    if (
      selectedPartnerName !== "" &&
      !response.partners.some((partner) => partner.name === selectedPartnerName)
    ) {
      setSelectedPartnerName("");
      setPartnerUpdateForm(initialPartnerUpdateForm);
      setPartnerDeleteName("");
    }
    setLastLoadedAt(Date.now());
    return response.partners.length;
  }

  function applyBoardToForms(nextBoard: PartnerBoardConfig) {
    const target = nextBoard.target;
    if (target) {
      setTargetForm({
        type:
          target.type === "webhook_message"
            ? "webhook_message"
            : "channel_message",
        messageID: target.message_id ?? "",
        webhookURL: target.webhook_url ?? "",
        channelID: target.channel_id ?? "",
      });
    } else {
      setTargetForm(initialTargetForm);
    }

    const template = nextBoard.template;
    if (template) {
      setTemplateForm({
        title: template.title ?? "",
        intro: template.intro ?? "",
        sectionHeaderTemplate: template.section_header_template ?? "",
        lineTemplate: template.line_template ?? "",
        emptyStateText: template.empty_state_text ?? "",
      });
    } else {
      setTemplateForm(initialTemplateForm);
    }
  }

  if (appView === "landing") {
    return (
      <main className="shell">
        <div className="app-frame landing-frame">
          <header className="site-topbar">
            <div className="site-brand" aria-hidden="true">
              <img className="site-brand-icon" src={siteBrandIconSrc} alt="" />
            </div>

            <nav className="site-nav" aria-label="Primary">
              {landingNavLinks.map((link) => (
                <button
                  key={link.id}
                  className={[
                    "site-nav-link",
                    link.id === "premium" ? "site-nav-link-accent" : "",
                  ]
                    .filter(Boolean)
                    .join(" ")}
                  type="button"
                  onClick={() => openLandingSection(link.id)}
                >
                  {link.label}
                </button>
              ))}
            </nav>

            <div className="site-actions">
              {authState === "checking" ? (
                <span className="inline-badge badge-muted">Checking session</span>
              ) : null}
              {authState === "oauth_unavailable" ? (
                <button className="site-action site-action-secondary" type="button" disabled>
                  OAuth unavailable
                </button>
              ) : null}
              {authState === "signed_out" ? (
                <button
                  className="site-action site-action-primary"
                  type="button"
                  disabled={loading}
                  onClick={() => void beginLogin(dashboardHomePath)}
                >
                  Login with Discord
                </button>
              ) : null}
              {authState === "signed_in" ? (
                <>
                  <button
                    className="site-action site-action-secondary"
                    type="button"
                    disabled={loading}
                    onClick={() => navigateToAppPath(controlPanelPath)}
                  >
                    Control Panel
                  </button>
                  <button
                    className="site-action site-action-secondary"
                    type="button"
                    disabled={loading}
                    onClick={() => void logout()}
                  >
                    Logout
                  </button>
                </>
              ) : null}
            </div>
          </header>

          <section className="landing-hero">
            <div className="landing-decor landing-decor-cross landing-decor-a" aria-hidden="true" />
            <div className="landing-decor landing-decor-cross landing-decor-b" aria-hidden="true" />
            <div className="landing-decor landing-decor-dot landing-decor-c" aria-hidden="true" />
            <div className="landing-decor landing-decor-dot landing-decor-d" aria-hidden="true" />
            <div className="landing-decor landing-decor-ring landing-decor-e" aria-hidden="true" />
            <div className="landing-decor landing-decor-ring landing-decor-f" aria-hidden="true" />
            <div className="landing-decor landing-decor-bar landing-decor-g" aria-hidden="true" />
            <div className="landing-decor landing-decor-bar landing-decor-h" aria-hidden="true" />

            <div className="landing-hero-copy">
              <p className="eyebrow">Discord automation platform</p>
              <h1>Keep the homepage public. Keep management in the control panel.</h1>
              <p className="hero-text landing-hero-text">
                This landing page stays intentionally free of guild selectors, sync actions,
                and board editors. Authentication happens with Discord in the top bar, then
                the dashboard opens only when the user explicitly enters the control panel.
              </p>
            </div>

            <div className="landing-pill-grid" aria-label="Landing page highlights">
              <article className="landing-pill-card">
                <span className="metric-label">Session</span>
                <strong>{signedInUserName ?? formatAuthStateLabel(authState)}</strong>
                <small>
                  {session !== null
                    ? `${manageableGuilds.length} manageable guild${manageableGuilds.length === 1 ? "" : "s"} available.`
                    : formatAuthSupportText(authState, manageableGuilds.length)}
                </small>
              </article>
              <article className="landing-pill-card emphasis">
                <span className="metric-label">Control panel access</span>
                <strong>
                  {authState === "signed_in"
                    ? "Unlocked after sign-in"
                    : authState === "oauth_unavailable"
                      ? "Waiting on OAuth configuration"
                      : "Discord OAuth gated"}
                </strong>
                <small>
                  Dashboard actions stay off the homepage and open only after a deliberate
                  route change into the control panel.
                </small>
              </article>
            </div>
          </section>

          <section id="about" className="landing-panel">
            <div className="landing-section-heading">
              <p className="section-kicker">About</p>
              <h2>A cleaner separation between public website content and guild operations.</h2>
              <p className="section-text">
                The homepage now behaves like a proper product front door: simple navigation,
                stronger visual hierarchy, and no operational clutter fighting for attention.
              </p>
            </div>

            <div className="landing-about-grid">
              <article className="landing-story-card">
                <strong>Landing first</strong>
                <p>
                  Visitors see a stable website shell with marketing content, trust cues,
                  and public navigation instead of configuration forms.
                </p>
              </article>
              <article className="landing-story-card">
                <strong>Dashboard second</strong>
                <p>
                  Partner board configuration, delivery settings, and data management stay
                  grouped under the control panel route.
                </p>
              </article>
            </div>
          </section>

          <section id="features" className="landing-panel">
            <div className="landing-section-heading">
              <p className="section-kicker">Features</p>
              <h2>Production-minded UX choices for large-scale products.</h2>
            </div>

            <div className="landing-feature-grid">
              {landingFeatureCards.map((card) => (
                <article key={card.title} className="landing-feature-card">
                  <h3>{card.title}</h3>
                  <p>{card.body}</p>
                </article>
              ))}
            </div>
          </section>

          <section id="status" className="landing-panel">
            <div className="landing-section-heading">
              <p className="section-kicker">Status</p>
              <h2>Operational signals remain visible without exposing admin controls.</h2>
            </div>

            <div className="landing-status-grid">
              <article className="landing-story-card">
                <span className="summary-label">Auth state</span>
                <strong>{formatAuthStateLabel(authState)}</strong>
                <p>{formatAuthSupportText(authState, manageableGuilds.length)}</p>
              </article>
              <article className="landing-story-card">
                <span className="summary-label">Serving path</span>
                <strong>{dashboardHomePath}</strong>
                <p>The landing and control-panel views stay inside the current embedded dashboard base path.</p>
              </article>
              <article className="landing-story-card">
                <span className="summary-label">Trust rail</span>
                <div className="landing-trust-list">
                  {landingTrustPoints.map((point) => (
                    <span key={point} className="fandom-chip">
                      {point}
                    </span>
                  ))}
                </div>
              </article>
            </div>
          </section>

          <section id="invite" className="landing-panel">
            <div className="landing-section-heading">
              <p className="section-kicker">Public entry points</p>
              <h2>Reserve the homepage for onboarding, docs, support, and premium paths.</h2>
            </div>

            <div className="landing-entry-grid">
              <article className="landing-entry-card">
                <span className="summary-label">Add to server</span>
                <h3>Wire the invite flow here.</h3>
                <p>Keep bot onboarding on the public page instead of inside the dashboard.</p>
              </article>
              <article id="documentation" className="landing-entry-card">
                <span className="summary-label">Documentation</span>
                <h3>Route users to docs and command references.</h3>
                <p>Documentation belongs in the site navigation, not in the configuration workspace.</p>
              </article>
              <article id="support" className="landing-entry-card">
                <span className="summary-label">Support</span>
                <h3>Expose help without mixing it with admin tooling.</h3>
                <p>Support calls-to-action stay predictable across desktop and mobile layouts.</p>
              </article>
              <article id="premium" className="landing-entry-card">
                <span className="summary-label">Premium</span>
                <h3>Keep monetization visible but separate.</h3>
                <p>Pricing and upsell content can live here without leaking any dashboard semantics.</p>
              </article>
            </div>
          </section>

          <footer className={`status-banner status-${status.kind} landing-status-banner`} aria-live="polite">
            <div className="status-copy">
              <p className="status-kicker">{formatStatusLabel(status.kind)}</p>
              <strong>{status.message}</strong>
            </div>
            <div className="status-meta">
              {loading ? (
                <span className="status-pill">{busyAction || "Working..."}</span>
              ) : null}
              <span className="status-secondary">
                {session !== null
                  ? `Signed in as ${formatUserLabel(session)}.`
                  : "Public landing page loaded."}
              </span>
            </div>
          </footer>
        </div>
      </main>
    );
  }

  return (
    <main className="shell control-shell">
      <div className="app-frame">
        <header className="topbar">
          <div className="brand-lockup">
            <div className="brand-mark" aria-hidden="true">
              <span>PB</span>
            </div>
            <div className="brand-copy">
              <p className="eyebrow">Control panel</p>
              <h1>Partner Board</h1>
              <p className="topbar-note">{controlPanelPath}</p>
            </div>
          </div>

          <div className="topbar-actions">
            <div className="theme-chip">
              <span>{selectedGuild?.name ?? formatAuthStateLabel(authState)}</span>
              <small>{activeOriginLabel}</small>
            </div>
            <button
              className="button-secondary button-compact"
              type="button"
              disabled={loading}
              onClick={() => void refreshSession()}
            >
              Refresh
            </button>
            {authState === "signed_out" ? (
              <button
                className="button-primary button-compact"
                type="button"
                disabled={loading}
                onClick={() => void beginLogin(controlPanelPath)}
              >
                Login
              </button>
            ) : null}
            {authState === "signed_in" ? (
              <>
                <button
                  className="button-outline button-compact"
                  type="button"
                  disabled={loading}
                  onClick={() => scrollToSection("overview")}
                >
                  Focus
                </button>
                <button
                  className="button-outline button-compact"
                  type="button"
                  disabled={loading}
                  onClick={() => void logout()}
                >
                  Sign out
                </button>
              </>
            ) : null}
          </div>
        </header>

        <section className="hero">
          <div className="hero-orb hero-orb-a" aria-hidden="true" />
          <div className="hero-orb hero-orb-b" aria-hidden="true" />
          <div className="hero-orb hero-orb-c" aria-hidden="true" />

          <div className="hero-grid">
            <div className="hero-copy">
              <div>
                <p className="eyebrow">Workspace</p>
                <h2>Delivery. Template. Partners.</h2>
                <p className="hero-text">Compact editing for the live board.</p>
              </div>

              <div className="hero-actions">
                {authState === "signed_in" ? (
                  <>
                    <button
                      className="button-primary button-compact"
                      type="button"
                      disabled={!canManageGuild}
                      onClick={() => void refreshBoard()}
                    >
                      Load
                    </button>
                    <button
                      className="button-ghost button-compact"
                      type="button"
                      disabled={loading}
                      onClick={() => scrollToSection("partners")}
                    >
                      Partners
                    </button>
                  </>
                ) : (
                  <button
                    className="button-primary button-compact"
                    type="button"
                    disabled={loading}
                    onClick={() => void beginLogin(controlPanelPath)}
                  >
                    Login
                  </button>
                )}
              </div>

              <div className="metric-grid" aria-label="Workspace metrics">
                <article className="metric-card emphasis">
                  <span className="metric-label">Ready</span>
                  <strong>{readinessScore}%</strong>
                  <small>{completedSteps}/{workflowSteps.length}</small>
                </article>
                <article className="metric-card">
                  <span className="metric-label">Guilds</span>
                  <strong>{manageableGuilds.length}</strong>
                  <small>
                    {selectedGuild === null
                      ? "No selection"
                      : selectedGuild.name}
                  </small>
                </article>
                <article className="metric-card">
                  <span className="metric-label">Partners</span>
                  <strong>{filteredPartners.length}</strong>
                  <small>
                    {deferredPartnerSearch === ""
                      ? "All"
                      : "Filtered"}
                  </small>
                </article>
                <article className="metric-card">
                  <span className="metric-label">Loaded</span>
                  <strong>{formatLastLoadedAt(lastLoadedAt)}</strong>
                  <small>
                    {board === null
                      ? "Not yet"
                      : "Snapshot"}
                  </small>
                </article>
              </div>
            </div>

            <aside className="hero-panel">
              <div className="hero-panel-header">
                <div className="identity-cluster">
                  <div className="identity-avatar" aria-hidden="true">
                    {sessionAvatarURL !== null ? (
                      <img src={sessionAvatarURL} alt="" />
                    ) : (
                      <span>
                        {session !== null
                          ? getInitials(formatSessionTitle(session))
                          : "?"}
                      </span>
                    )}
                  </div>
                  <div className="hero-panel-copy">
                    <p className="eyebrow">Session</p>
                    <h3>
                      {session !== null
                        ? formatSessionTitle(session)
                        : formatAuthStateLabel(authState)}
                    </h3>
                    <p>
                      {session !== null
                        ? session.user.id
                        : formatAuthSupportText(authState, manageableGuilds.length)}
                    </p>
                  </div>
                </div>
                <span className={`status-chip status-chip-${status.kind}`}>
                  {formatStatusLabel(status.kind)}
                </span>
              </div>

              <div className="hero-panel-meta">
                <article className="meta-card">
                  <span>Active origin</span>
                  <strong>{activeOriginLabel}</strong>
                </article>
                <article className="meta-card">
                  <span>Selected guild</span>
                  <strong>
                    {selectedGuild === null ? "No guild selected" : selectedGuild.name}
                  </strong>
                </article>
              </div>

              <div className="workflow-card">
                <div className="card-heading">
                  <div>
                    <p className="section-kicker">Flow</p>
                    <h3>Next</h3>
                  </div>
                  {nextStep !== null ? (
                    <button
                      className="button-ghost button-compact"
                      type="button"
                      onClick={() => scrollToSection(nextStep.sectionId)}
                    >
                      Open
                    </button>
                  ) : null}
                </div>

                <ol className="workflow-list">
                  {workflowSteps.map((step, index) => (
                    <li
                      key={step.id}
                      className={[
                        "workflow-item",
                        step.completed ? "is-complete" : "",
                        !step.completed && step.current ? "is-current" : "",
                      ]
                        .filter(Boolean)
                        .join(" ")}
                    >
                      <div className="workflow-index">{index + 1}</div>
                      <div className="workflow-copy">
                        <div className="workflow-title-row">
                          <strong>{step.title}</strong>
                          <span className="workflow-state">
                            {step.completed
                              ? "Done"
                              : step.current
                                ? "Current"
                                : "Pending"}
                          </span>
                        </div>
                      </div>
                    </li>
                  ))}
                </ol>
              </div>
            </aside>
          </div>
        </section>

        <nav className="section-nav" aria-label="Dashboard sections">
          {sectionLinks.map((link) => (
            <button
              key={link.id}
              className="nav-chip"
              type="button"
              onClick={() => scrollToSection(link.id)}
            >
              {link.label}
            </button>
          ))}
        </nav>

        <div className="workspace-layout">
          <div className="workspace-main">
            <section id="overview" className="card surface-card">
              <div className="card-heading">
                <div>
                  <p className="section-kicker">Workspace</p>
                  <h2>Connection</h2>
                </div>
                <div className="badge-row">
                  <span
                    className={[
                      "inline-badge",
                      authState === "signed_in"
                        ? "badge-success"
                        : "badge-muted",
                    ].join(" ")}
                  >
                    {formatAuthStateLabel(authState)}
                  </span>
                  {selectedGuild !== null ? (
                    <span className="inline-badge badge-strong">
                      {selectedGuild.name}
                    </span>
                  ) : null}
                </div>
              </div>

              <div className="overview-grid">
                <div className="form-stack">
                  <label>
                    <span className="field-label">Endpoint</span>
                    <input
                      value={baseUrlInput}
                      onChange={(event) => setBaseUrlInput(event.target.value)}
                      placeholder="Same origin or https://control.example.com"
                    />
                  </label>

                  <div className="inline-actions">
                    <button
                      className="button-primary button-compact"
                      type="button"
                      disabled={loading || !baseUrlDirty}
                      onClick={applyBaseUrl}
                    >
                      Apply
                    </button>
                    <p className="helper-text">
                      {baseUrlDirty ? "Draft" : activeOriginLabel}
                    </p>
                  </div>

                  <label>
                    <span className="field-label">Guild</span>
                    <select
                      value={guildID}
                      onChange={(event) => handleGuildSelection(event.target.value)}
                      disabled={
                        authState !== "signed_in" || manageableGuilds.length === 0
                      }
                    >
                      <option value="">
                        {authState !== "signed_in"
                          ? "Sign in to load guilds"
                          : "Select a guild"}
                      </option>
                      {manageableGuilds.map((guild) => (
                        <option key={guild.id} value={guild.id}>
                          {guild.name} ({guild.id})
                        </option>
                      ))}
                    </select>
                  </label>

                  <div className="actions">
                    <button
                      className="button-primary button-compact"
                      type="button"
                      disabled={!canManageGuild}
                      onClick={() => void refreshBoard()}
                    >
                      Load
                    </button>
                    <button
                      className="button-secondary button-compact"
                      type="button"
                      disabled={!canManageGuild}
                      onClick={() => void refreshPartnersOnly()}
                    >
                      Partners
                    </button>
                    <button
                      className="button-secondary button-compact"
                      type="button"
                      disabled={!canManageGuild}
                      onClick={() => void syncBoard()}
                    >
                      Sync
                    </button>
                  </div>
                </div>

                <div className="overview-side">
                  <section className="identity-card">
                    <div className="identity-cluster">
                      <div className="identity-avatar" aria-hidden="true">
                        {sessionAvatarURL !== null ? (
                          <img src={sessionAvatarURL} alt="" />
                        ) : (
                          <span>
                            {session !== null
                              ? getInitials(formatSessionTitle(session))
                              : "?"}
                          </span>
                        )}
                      </div>
                      <div className="hero-panel-copy">
                        <p className="eyebrow">Operator</p>
                        <h3>
                          {session !== null
                            ? formatUserLabel(session)
                            : formatAuthStateLabel(authState)}
                        </h3>
                        <p>
                          {formatAuthSupportText(authState, manageableGuilds.length)}
                        </p>
                      </div>
                    </div>
                  </section>

                  <div className="overview-callouts">
                    <article className="mini-panel">
                      <div className="mini-panel-header">
                        <div>
                          <p className="section-kicker">Active server</p>
                          <strong>{activeOriginLabel}</strong>
                        </div>
                        <div className="mini-panel-avatar" aria-hidden="true">
                          <span>API</span>
                        </div>
                      </div>
                      <p>
                        Requests, login redirects, and state changes all use the
                        currently applied origin.
                      </p>
                    </article>

                    <article className="mini-panel">
                      <div className="mini-panel-header">
                        <div>
                          <p className="section-kicker">Selected guild</p>
                          <strong>
                            {selectedGuild === null
                              ? "No guild selected"
                              : selectedGuild.name}
                          </strong>
                        </div>
                        <div className="mini-panel-avatar" aria-hidden="true">
                          {selectedGuildIcon !== null ? (
                            <img src={selectedGuildIcon} alt="" />
                          ) : (
                            <span>
                              {selectedGuild !== null
                                ? getInitials(selectedGuild.name)
                                : "?"}
                            </span>
                          )}
                        </div>
                      </div>
                      <p>
                        {selectedGuild === null
                          ? "Pick a guild."
                          : `${selectedGuild.owner ? "Owner" : "Manage Server"} · ${selectedGuild.id}`}
                      </p>
                    </article>
                  </div>
                </div>
              </div>
            </section>

            <section id="delivery" className="card surface-card">
              <div className="card-heading">
                <div>
                  <p className="section-kicker">Delivery</p>
                  <h2>Target</h2>
                </div>
                <div className="badge-row">
                  <span className="inline-badge badge-strong">
                    {targetForm.type === "webhook_message"
                      ? "Webhook"
                      : "Channel"}
                  </span>
                  <span
                    className={[
                      "inline-badge",
                      targetConfigured ? "badge-success" : "badge-muted",
                    ].join(" ")}
                  >
                    {targetConfigured ? "Ready" : "Draft"}
                  </span>
                </div>
              </div>

              <div className="summary-grid">
                <article className="summary-tile">
                  <span className="summary-label">Mode</span>
                  <strong>
                    {targetForm.type === "webhook_message"
                      ? "Webhook"
                      : "Channel"}
                  </strong>
                  <small>{targetConfigured ? "Ready" : "Needs fields"}</small>
                </article>
                <article className="summary-tile">
                  <span className="summary-label">Message ID</span>
                  <strong>
                    {targetDraft.message_id?.trim() || "Unset"}
                  </strong>
                  <small>Board message</small>
                </article>
                <article className="summary-tile">
                  <span className="summary-label">Destination</span>
                  <strong>{summarizeTarget(targetDraft)}</strong>
                  <small>
                    {targetForm.type === "webhook_message"
                      ? "Webhook URL"
                      : "Channel ID"}
                  </small>
                </article>
              </div>

              <div className="form-grid form-grid-two">
                <label>
                  <span className="field-label">Target type</span>
                  <select
                    value={targetForm.type}
                    onChange={(event) =>
                      setTargetForm((prev) => ({
                        ...prev,
                        type: event.target.value as
                          | "webhook_message"
                          | "channel_message",
                      }))
                    }
                    disabled={authState !== "signed_in"}
                  >
                    <option value="channel_message">channel_message</option>
                    <option value="webhook_message">webhook_message</option>
                  </select>
                </label>

                <label>
                  <span className="field-label">Message ID</span>
                  <input
                    value={targetForm.messageID}
                    onChange={(event) =>
                      setTargetForm((prev) => ({
                        ...prev,
                        messageID: event.target.value,
                      }))
                    }
                    placeholder="123456789012345678"
                    disabled={authState !== "signed_in"}
                  />
                </label>

                <label>
                  <span className="field-label">Channel ID</span>
                  <input
                    value={targetForm.channelID}
                    onChange={(event) =>
                      setTargetForm((prev) => ({
                        ...prev,
                        channelID: event.target.value,
                      }))
                    }
                    placeholder="channel_message"
                    disabled={
                      authState !== "signed_in" ||
                      targetForm.type === "webhook_message"
                    }
                  />
                </label>

                <label>
                  <span className="field-label">Webhook URL</span>
                  <input
                    value={targetForm.webhookURL}
                    onChange={(event) =>
                      setTargetForm((prev) => ({
                        ...prev,
                        webhookURL: event.target.value,
                      }))
                    }
                    placeholder="webhook_message"
                    disabled={
                      authState !== "signed_in" ||
                      targetForm.type === "channel_message"
                    }
                  />
                </label>
              </div>

              <div className="actions">
                <button
                  className="button-primary button-compact"
                  type="button"
                  disabled={!canManageGuild}
                  onClick={() => void saveTarget()}
                >
                  Save
                </button>
              </div>
            </section>

            <section id="template" className="card surface-card">
              <div className="card-heading">
                <div>
                  <p className="section-kicker">Template</p>
                  <h2>Copy</h2>
                </div>
                <div className="badge-row">
                  <span className="inline-badge badge-strong">
                    {templateCompletion}/{templateFieldCount}
                  </span>
                  <span
                    className={[
                      "inline-badge",
                      templateConfigured ? "badge-success" : "badge-muted",
                    ].join(" ")}
                  >
                    {templateConfigured ? "Ready" : "Draft"}
                  </span>
                </div>
              </div>

              <div className="summary-grid">
                <article className="summary-tile">
                  <span className="summary-label">Board title</span>
                  <strong>{templateDraft.title?.trim() || "Untitled"}</strong>
                  <small>Header</small>
                </article>
                <article className="summary-tile">
                  <span className="summary-label">Intro</span>
                  <strong>
                    {templateDraft.intro?.trim() !== ""
                      ? "Present"
                      : "Empty"}
                  </strong>
                  <small>Optional</small>
                </article>
                <article className="summary-tile">
                  <span className="summary-label">State</span>
                  <strong>{summarizeTemplate(templateDraft)}</strong>
                  <small>Live draft</small>
                </article>
              </div>

              <div className="form-grid form-grid-two">
                <label>
                  <span className="field-label">Title</span>
                  <input
                    value={templateForm.title}
                    onChange={(event) =>
                      setTemplateForm((prev) => ({
                        ...prev,
                        title: event.target.value,
                      }))
                    }
                    placeholder="Partner Board"
                    disabled={authState !== "signed_in"}
                  />
                </label>

                <label>
                  <span className="field-label">Section header template</span>
                  <input
                    value={templateForm.sectionHeaderTemplate}
                    onChange={(event) =>
                      setTemplateForm((prev) => ({
                        ...prev,
                        sectionHeaderTemplate: event.target.value,
                      }))
                    }
                    placeholder="Section header"
                    disabled={authState !== "signed_in"}
                  />
                </label>

                <label>
                  <span className="field-label">Line template</span>
                  <input
                    value={templateForm.lineTemplate}
                    onChange={(event) =>
                      setTemplateForm((prev) => ({
                        ...prev,
                        lineTemplate: event.target.value,
                      }))
                    }
                    placeholder="Partner row"
                    disabled={authState !== "signed_in"}
                  />
                </label>

                <label>
                  <span className="field-label">Empty state text</span>
                  <input
                    value={templateForm.emptyStateText}
                    onChange={(event) =>
                      setTemplateForm((prev) => ({
                        ...prev,
                        emptyStateText: event.target.value,
                      }))
                    }
                    placeholder="No partners"
                    disabled={authState !== "signed_in"}
                  />
                </label>
              </div>

              <label className="full-width">
                <span className="field-label">Intro</span>
                <textarea
                  rows={5}
                  value={templateForm.intro}
                  onChange={(event) =>
                    setTemplateForm((prev) => ({
                      ...prev,
                      intro: event.target.value,
                    }))
                  }
                  placeholder="Optional"
                  disabled={authState !== "signed_in"}
                />
              </label>

              <div className="actions">
                <button
                  className="button-primary button-compact"
                  type="button"
                  disabled={!canManageGuild}
                  onClick={() => void saveTemplate()}
                >
                  Save
                </button>
              </div>
            </section>
          </div>

          <aside className="workspace-sidebar">
            <section className="card side-card">
              <div className="card-heading">
                <div>
                  <p className="section-kicker">Snapshot</p>
                  <h3>State</h3>
                </div>
              </div>

              {board === null ? (
                <div className="empty-panel">
                  <strong>No snapshot</strong>
                  <p>Load the board.</p>
                </div>
              ) : (
                <>
                  <div className="snapshot-grid">
                    <article className="snapshot-stat">
                      <span className="summary-label">Delivery</span>
                      <strong>{targetConfigured ? "Ready" : "Incomplete"}</strong>
                    </article>
                    <article className="snapshot-stat">
                      <span className="summary-label">Template</span>
                      <strong>
                        {templateConfigured ? "Structured" : "Needs attention"}
                      </strong>
                    </article>
                    <article className="snapshot-stat">
                      <span className="summary-label">Partners</span>
                      <strong>{partners.length}</strong>
                    </article>
                    <article className="snapshot-stat">
                      <span className="summary-label">Loaded</span>
                      <strong>{formatLastLoadedAt(lastLoadedAt)}</strong>
                    </article>
                  </div>

                  <div className="snapshot-block">
                    <p>Delivery</p>
                    <strong>{summarizeTarget(targetDraft)}</strong>
                    <span>
                      {targetConfigured
                        ? "Ready"
                        : "Needs fields"}
                    </span>
                  </div>

                  <div className="snapshot-block">
                    <p>Template</p>
                    <strong>{templateDraft.title?.trim() || "Untitled board"}</strong>
                    <span>{summarizeTemplate(templateDraft)}</span>
                  </div>

                  <div className="chip-cloud">
                    {fandomHighlights.length === 0 ? (
                      <span className="fandom-chip">No fandom groups yet</span>
                    ) : (
                      fandomHighlights.map((highlight) => (
                        <span key={highlight.label} className="fandom-chip">
                          {highlight.label} - {highlight.count}
                        </span>
                      ))
                    )}
                  </div>
                </>
              )}
            </section>

            <section className="card side-card">
              <div className="card-heading">
                <div>
                  <p className="section-kicker">Flow</p>
                  <h3>Checklist</h3>
                </div>
              </div>

              <div className="checklist">
                {workflowSteps.map((step) => (
                  <button
                    key={step.id}
                    className={[
                      "checklist-item",
                      step.completed ? "is-complete" : "",
                      !step.completed && step.current ? "is-current" : "",
                    ]
                      .filter(Boolean)
                      .join(" ")}
                    type="button"
                    onClick={() => scrollToSection(step.sectionId)}
                    >
                      <span className="checklist-copy">
                        <span className="checklist-label">{step.title}</span>
                      </span>
                      <span className="workflow-state">
                        {step.completed
                        ? "Done"
                        : step.current
                          ? "Current"
                          : "Pending"}
                    </span>
                  </button>
                ))}
              </div>
            </section>

            <section className="card side-card">
              <div className="card-heading">
                <div>
                  <p className="section-kicker">Context</p>
                  <h3>Scope</h3>
                </div>
              </div>

              <dl className="context-list">
                <div>
                  <dt>Dashboard path</dt>
                  <dd>/dashboard/</dd>
                </div>
                <div>
                  <dt>Active origin</dt>
                  <dd>{activeOriginLabel}</dd>
                </div>
                <div>
                  <dt>OAuth scopes</dt>
                  <dd>
                    {session !== null && session.scopes.length > 0
                      ? session.scopes.join(", ")
                      : "After sign-in"}
                  </dd>
                </div>
                <div>
                  <dt>Guild filtering</dt>
                  <dd>Manageable + bot joined</dd>
                </div>
              </dl>
            </section>
          </aside>
        </div>

        <section id="partners" className="partner-workspace">
          <section className="card partner-table-card">
            <div className="card-heading partner-heading">
              <div>
                <p className="section-kicker">Partners</p>
                <h2>Directory</h2>
              </div>

              <div className="partner-toolbar">
                <label className="search-field">
                  <span className="field-label">Filter</span>
                  <input
                    value={partnerSearch}
                    onChange={(event) => setPartnerSearch(event.target.value)}
                    placeholder="Search"
                  />
                </label>
                <span className="inline-badge badge-strong">
                  {filteredPartnerLabel}
                </span>
              </div>
            </div>

            {partners.length === 0 ? (
              <div className="empty-panel">
                <strong>No partners</strong>
                <p>Add the first row.</p>
              </div>
            ) : filteredPartners.length === 0 ? (
              <div className="empty-panel">
                <strong>No matches</strong>
                <p>Clear the filter.</p>
              </div>
            ) : (
              <div className="table-shell">
                <table className="partner-table">
                  <thead>
                    <tr>
                      <th>Fandom</th>
                      <th>Name</th>
                      <th>Link</th>
                      <th>Action</th>
                    </tr>
                  </thead>
                  <tbody>
                    {filteredPartners.map((partner) => (
                      <tr
                        key={`${partner.name}|${partner.link}`}
                        className={
                          selectedPartnerName === partner.name ? "is-selected" : ""
                        }
                      >
                        <td>{partner.fandom?.trim() || "Other"}</td>
                        <td>{partner.name}</td>
                        <td>
                          <a
                            className="partner-link"
                            href={partner.link}
                            target="_blank"
                            rel="noreferrer"
                          >
                            {partner.link}
                          </a>
                        </td>
                        <td>
                          <button
                            className="button-secondary partner-row-action"
                            type="button"
                            disabled={authState !== "signed_in"}
                            onClick={() => loadPartnerIntoEditor(partner)}
                          >
                            Edit
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </section>

          <div id="partner-editor" className="partner-actions-grid">
            <section className="card">
              <div className="editor-header">
                <div className="editor-title">
                  <p className="section-kicker">Add</p>
                  <h3>New</h3>
                </div>
              </div>

              <div className="form-grid">
                <label>
                  <span className="field-label">Fandom</span>
                  <input
                    value={partnerForm.fandom}
                    onChange={(event) =>
                      setPartnerForm((prev) => ({
                        ...prev,
                        fandom: event.target.value,
                      }))
                    }
                    placeholder="Optional grouping label"
                    disabled={authState !== "signed_in"}
                  />
                </label>

                <label>
                  <span className="field-label">Name</span>
                  <input
                    value={partnerForm.name}
                    onChange={(event) =>
                      setPartnerForm((prev) => ({
                        ...prev,
                        name: event.target.value,
                      }))
                    }
                    placeholder="Partner name"
                    disabled={authState !== "signed_in"}
                  />
                </label>

                <label>
                  <span className="field-label">Link</span>
                  <input
                    value={partnerForm.link}
                    onChange={(event) =>
                      setPartnerForm((prev) => ({
                        ...prev,
                        link: event.target.value,
                      }))
                    }
                    placeholder="https://discord.gg/example"
                    disabled={authState !== "signed_in"}
                  />
                </label>
              </div>

              <div className="actions">
                <button
                  className="button-primary button-compact"
                  type="button"
                  disabled={!canManageGuild}
                  onClick={() => void addPartner()}
                >
                  Add
                </button>
              </div>
            </section>

            <section className="card">
              <div className="editor-header">
                <div className="editor-title">
                  <p className="section-kicker">Edit</p>
                  <h3>Existing</h3>
                </div>
                {selectedPartnerName !== "" ? (
                  <span className="selected-tag">{selectedPartnerName}</span>
                ) : null}
              </div>

              <div className="form-grid">
                <label>
                  <span className="field-label">Current name</span>
                  <input
                    value={partnerUpdateForm.currentName}
                    onChange={(event) =>
                      setPartnerUpdateForm((prev) => ({
                        ...prev,
                        currentName: event.target.value,
                      }))
                    }
                    placeholder="Name of the partner to update"
                    disabled={authState !== "signed_in"}
                  />
                </label>

                <label>
                  <span className="field-label">Fandom</span>
                  <input
                    value={partnerUpdateForm.fandom}
                    onChange={(event) =>
                      setPartnerUpdateForm((prev) => ({
                        ...prev,
                        fandom: event.target.value,
                      }))
                    }
                    placeholder="Optional grouping label"
                    disabled={authState !== "signed_in"}
                  />
                </label>

                <label>
                  <span className="field-label">Name</span>
                  <input
                    value={partnerUpdateForm.name}
                    onChange={(event) =>
                      setPartnerUpdateForm((prev) => ({
                        ...prev,
                        name: event.target.value,
                      }))
                    }
                    placeholder="Updated partner name"
                    disabled={authState !== "signed_in"}
                  />
                </label>

                <label>
                  <span className="field-label">Link</span>
                  <input
                    value={partnerUpdateForm.link}
                    onChange={(event) =>
                      setPartnerUpdateForm((prev) => ({
                        ...prev,
                        link: event.target.value,
                      }))
                    }
                    placeholder="https://discord.gg/example"
                    disabled={authState !== "signed_in"}
                  />
                </label>
              </div>

              <div className="actions">
                <button
                  className="button-primary button-compact"
                  type="button"
                  disabled={!canManageGuild}
                  onClick={() => void updatePartner()}
                >
                  Save
                </button>
              </div>
            </section>

            <section className="card">
              <div className="editor-header">
                <div className="editor-title">
                  <p className="section-kicker">Delete</p>
                  <h3>Remove</h3>
                </div>
              </div>

              <label>
                <span className="field-label">Partner name</span>
                <input
                  value={partnerDeleteName}
                  onChange={(event) => setPartnerDeleteName(event.target.value)}
                  placeholder="Exact partner name"
                  disabled={authState !== "signed_in"}
                />
              </label>

              <div className="actions">
                <button
                  className="button-danger button-compact"
                  type="button"
                  disabled={!canManageGuild}
                  onClick={() => void deletePartner()}
                >
                  Delete
                </button>
              </div>
            </section>
          </div>
        </section>

        <footer className={`status-banner status-${status.kind}`} aria-live="polite">
          <div className="status-copy">
            <p className="status-kicker">{formatStatusLabel(status.kind)}</p>
            <strong>{status.message}</strong>
          </div>
          <div className="status-meta">
            {loading ? (
              <span className="status-pill">{busyAction || "Working..."}</span>
            ) : null}
            <span className="status-secondary">
              {board === null
                ? "Board snapshot not loaded yet."
                : `Snapshot updated ${formatLastLoadedAt(lastLoadedAt)}.`}
            </span>
          </div>
        </footer>
      </div>
    </main>
  );
}

function formatError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

function formatSessionTitle(session: AuthSessionResponse): string {
  return (
    session.user.global_name?.trim() ||
    session.user.username.trim() ||
    session.user.id
  );
}

function formatUserLabel(session: AuthSessionResponse): string {
  return `${formatSessionTitle(session)} (${session.user.id})`;
}

function formatAuthStateLabel(authState: DashboardAuthState): string {
  switch (authState) {
    case "checking":
      return "Checking";
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

function formatAuthSupportText(
  authState: DashboardAuthState,
  manageableGuildCount: number,
): string {
  switch (authState) {
    case "checking":
      return "Checking session.";
    case "signed_out":
      return "Login required.";
    case "oauth_unavailable":
      return "OAuth unavailable.";
    case "signed_in":
      return `${manageableGuildCount} guild${manageableGuildCount === 1 ? "" : "s"}.`;
    default:
      return "Unknown session.";
  }
}

function formatStatusLabel(kind: StatusKind): string {
  switch (kind) {
    case "success":
      return "Success";
    case "error":
      return "Error";
    case "info":
      return "Info";
    case "idle":
    default:
      return "Ready";
  }
}

function formatLastLoadedAt(value: number | null): string {
  if (value === null) {
    return "Not yet";
  }

  return new Intl.DateTimeFormat(undefined, {
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value));
}

function resolveGuildSelection(
  currentGuildID: string,
  preferredGuildIDValue: string,
  guilds: ManageableGuild[],
): string {
  const availableGuildIDs = new Set(guilds.map((guild) => guild.id));
  if (currentGuildID.trim() !== "" && availableGuildIDs.has(currentGuildID.trim())) {
    return currentGuildID.trim();
  }
  if (
    preferredGuildIDValue.trim() !== "" &&
    availableGuildIDs.has(preferredGuildIDValue.trim())
  ) {
    return preferredGuildIDValue.trim();
  }
  if (guilds.length > 0) {
    return guilds[0].id;
  }
  return "";
}

function buildTargetPayload(form: TargetFormState): EmbedUpdateTargetConfig {
  const payload: EmbedUpdateTargetConfig = {
    type: form.type,
    message_id: form.messageID.trim(),
  };
  if (form.type === "webhook_message") {
    payload.webhook_url = form.webhookURL.trim();
  } else {
    payload.channel_id = form.channelID.trim();
  }
  return payload;
}

function buildTemplateDraft(
  form: TemplateFormState,
  currentTemplate?: PartnerBoardTemplateConfig,
): PartnerBoardTemplateConfig {
  return {
    ...(currentTemplate ?? {}),
    title: form.title.trim(),
    intro: form.intro,
    section_header_template: form.sectionHeaderTemplate,
    line_template: form.lineTemplate,
    empty_state_text: form.emptyStateText,
  };
}

function isTargetConfigured(target?: EmbedUpdateTargetConfig): boolean {
  if (target === undefined || target.message_id?.trim() === "") {
    return false;
  }
  if (target.type === "webhook_message") {
    return (target.webhook_url?.trim() ?? "") !== "";
  }
  return (target.channel_id?.trim() ?? "") !== "";
}

function isTemplateConfigured(template?: PartnerBoardTemplateConfig): boolean {
  if (template === undefined) {
    return false;
  }
  return (
    (template.title?.trim() ?? "") !== "" &&
    (template.section_header_template?.trim() ?? "") !== "" &&
    (template.line_template?.trim() ?? "") !== "" &&
    (template.empty_state_text?.trim() ?? "") !== ""
  );
}

function countFilledTemplateFields(template?: PartnerBoardTemplateConfig): number {
  if (template === undefined) {
    return 0;
  }

  const trackedFields = [
    template.title,
    template.intro,
    template.section_header_template,
    template.line_template,
    template.empty_state_text,
  ];

  return trackedFields.filter((value) => (value?.trim() ?? "") !== "").length;
}

function summarizeTarget(target?: EmbedUpdateTargetConfig): string {
  if (target === undefined) {
    return "Unset";
  }
  if (!isTargetConfigured(target)) {
    return target.type === "webhook_message"
      ? "Webhook draft"
      : "Channel draft";
  }
  return target.type === "webhook_message" ? "Webhook" : "Channel";
}

function summarizeTemplate(template?: PartnerBoardTemplateConfig): string {
  if (template === undefined) {
    return "Unset";
  }

  if (isTemplateConfigured(template)) {
    return template.intro?.trim() !== "" ? "Ready + intro" : "Ready";
  }
  return template.intro?.trim() !== "" ? "Draft + intro" : "Draft";
}

function buildWorkflowSteps(
  authState: DashboardAuthState,
  guildID: string,
  boardLoaded: boolean,
  targetConfigured: boolean,
  templateConfigured: boolean,
  partnerCount: number,
): WorkflowStep[] {
  return [
    {
      id: "auth",
      title: "Authenticate",
      description: "Use Discord OAuth before the dashboard edits any guild.",
      completed: authState === "signed_in",
      current: authState !== "signed_in",
      sectionId: "overview",
    },
    {
      id: "guild",
      title: "Select a guild",
      description:
        "Choose the server you want to manage from the filtered guild list.",
      completed: guildID.trim() !== "",
      current: authState === "signed_in" && guildID.trim() === "",
      sectionId: "overview",
    },
    {
      id: "board",
      title: "Load the board",
      description:
        "Pull the latest board configuration before making changes.",
      completed: boardLoaded,
      current:
        authState === "signed_in" && guildID.trim() !== "" && !boardLoaded,
      sectionId: "overview",
    },
    {
      id: "delivery",
      title: "Confirm delivery and template",
      description:
        "Make sure the target and template draft cover the publishing basics.",
      completed: targetConfigured && templateConfigured,
      current: boardLoaded && (!targetConfigured || !templateConfigured),
      sectionId: "delivery",
    },
    {
      id: "partners",
      title: "Curate partner entries",
      description:
        "Review the live list, then add, update, or remove partner rows.",
      completed: partnerCount > 0,
      current:
        boardLoaded &&
        targetConfigured &&
        templateConfigured &&
        partnerCount === 0,
      sectionId: "partners",
    },
  ];
}

function collectFandomHighlights(
  partners: PartnerEntryConfig[],
): FandomHighlight[] {
  const counts = new Map<string, number>();
  for (const partner of partners) {
    const key = partner.fandom?.trim() || "Other";
    counts.set(key, (counts.get(key) ?? 0) + 1);
  }

  return Array.from(counts.entries())
    .map(([label, count]) => ({ label, count }))
    .sort((left, right) => right.count - left.count || left.label.localeCompare(right.label))
    .slice(0, 4);
}

function validateTargetForm(form: TargetFormState): string | null {
  if (form.messageID.trim() === "") {
    return "Message ID is required before saving the target.";
  }
  if (form.type === "webhook_message" && form.webhookURL.trim() === "") {
    return "Webhook URL is required for webhook_message targets.";
  }
  if (form.type === "channel_message" && form.channelID.trim() === "") {
    return "Channel ID is required for channel_message targets.";
  }
  return null;
}

function validatePartnerForm(form: PartnerFormState): string | null {
  if (form.name.trim() === "") {
    return "Partner name is required.";
  }
  if (form.link.trim() === "") {
    return "Partner link is required.";
  }
  return null;
}

function validatePartnerUpdateForm(form: PartnerUpdateFormState): string | null {
  if (form.currentName.trim() === "") {
    return "Current partner name is required before updating.";
  }
  if (form.name.trim() === "") {
    return "Updated partner name is required.";
  }
  if (form.link.trim() === "") {
    return "Updated partner link is required.";
  }
  return null;
}

function normalizeBaseUrlInput(raw: string): string {
  return raw.trim().replace(/\/+$/, "");
}

function isValidBaseUrl(raw: string): boolean {
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

function buildGuildIconURL(guild: ManageableGuild): string | null {
  if (!guild.icon) {
    return null;
  }
  return `https://cdn.discordapp.com/icons/${guild.id}/${guild.icon}.webp?size=128`;
}

function buildUserAvatarURL(user: DiscordOAuthUser): string | null {
  if (!user.avatar) {
    return null;
  }
  return `https://cdn.discordapp.com/avatars/${user.id}/${user.avatar}.webp?size=128`;
}

function getInitials(value: string): string {
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

function resolveAppView(path: string): AppView {
  return isControlPanelPath(path) ? "control-panel" : "landing";
}

function normalizeDashboardAppPath(path: string): string {
  const trimmed = path.trim();
  if (trimmed === "" || trimmed === "/dashboard") {
    return dashboardHomePath;
  }

  const withoutTrailingSlash = trimmed.replace(/\/+$/, "");
  if (
    withoutTrailingSlash === controlPanelPath ||
    withoutTrailingSlash.startsWith(`${controlPanelPath}/`)
  ) {
    return controlPanelPath;
  }
  if (
    withoutTrailingSlash === dashboardHomePath.replace(/\/+$/, "") ||
    withoutTrailingSlash.startsWith(`${dashboardHomePath}`)
  ) {
    return dashboardHomePath;
  }
  return dashboardHomePath;
}

function isControlPanelPath(path: string): boolean {
  const normalized = path.trim().replace(/\/+$/, "");
  return (
    normalized === controlPanelPath ||
    normalized.startsWith(`${controlPanelPath}/`)
  );
}
