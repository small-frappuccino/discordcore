import { useCallback, useEffect, useMemo, useState } from "react";
import {
  type AuthSessionResponse,
  type EmbedUpdateTargetConfig,
  type ManageableGuild,
  type PartnerBoardConfig,
  type PartnerBoardTemplateConfig,
  ControlApiClient,
} from "./api/control";

type StatusKind = "idle" | "success" | "error" | "info";
type DashboardAuthState =
  | "checking"
  | "signed_out"
  | "signed_in"
  | "oauth_unavailable";
type DashboardTheme = "investigadora-paranormal" | "forum-spook-shack";

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

interface ThemeOption {
  id: DashboardTheme;
  label: string;
  helper: string;
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
const themeStorageKey = "discordcore.dashboard.theme";
const themeOptions: ThemeOption[] = [
  {
    id: "investigadora-paranormal",
    label: "Investigadora Paranormal",
    helper: "Equilibrada e moderna",
  },
  {
    id: "forum-spook-shack",
    label: "Forum Spook Shack",
    helper: "Sombria e vibrante",
  },
];

export default function App() {
  const [baseUrl, setBaseUrl] = useState(defaultBaseUrl);
  const [theme, setTheme] = useState<DashboardTheme>(resolveInitialTheme);
  const [guildID, setGuildID] = useState(preferredGuildID);
  const [board, setBoard] = useState<PartnerBoardConfig | null>(null);
  const [targetForm, setTargetForm] = useState(initialTargetForm);
  const [templateForm, setTemplateForm] = useState(initialTemplateForm);
  const [partnerForm, setPartnerForm] = useState(initialPartnerForm);
  const [partnerUpdateForm, setPartnerUpdateForm] = useState(
    initialPartnerUpdateForm,
  );
  const [partnerDeleteName, setPartnerDeleteName] = useState("");
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

  const client = useMemo(
    () =>
      new ControlApiClient({
        baseUrl,
      }),
    [baseUrl],
  );

  const refreshSession = useCallback(async () => {
    setLoading(true);
    try {
      const probe = await client.getSessionStatus();
      if (probe.status === "oauth_unavailable") {
        setAuthState("oauth_unavailable");
        setSession(null);
        setManageableGuilds([]);
        resetLoadedBoard();
        setStatus({
          kind: "info",
          message:
            "Discord OAuth is not configured on this control server. The dashboard can load, but web configuration is unavailable until OAuth is enabled.",
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
          message:
            "Sign in with Discord to configure a guild through the dashboard.",
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
          message:
            "Signed in, but there are no guilds you can manage with this bot.",
        });
        return;
      }

      setStatus({
        kind: "success",
        message: `Signed in as ${formatUserLabel(probe.session)}.`,
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
    } finally {
      setLoading(false);
    }
  }, [client]);

  useEffect(() => {
    void refreshSession();
  }, [refreshSession]);

  useEffect(() => {
    document.documentElement.setAttribute("data-theme", theme);
    window.localStorage.setItem(themeStorageKey, theme);
  }, [theme]);

  const partners = board?.partners ?? [];
  const canManageGuild =
    authState === "signed_in" && guildID.trim() !== "" && !loading;

  async function logout() {
    setLoading(true);
    try {
      await client.logout();
      setAuthState("signed_out");
      setSession(null);
      setManageableGuilds([]);
      resetLoadedBoard();
      setStatus({
        kind: "info",
        message: "Signed out. Sign in again to continue editing guild settings.",
      });
    } catch (error) {
      setStatus({
        kind: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
    }
  }

  async function refreshBoard() {
    if (!ensureGuildSelected()) {
      return;
    }

    const trimmedGuild = guildID.trim();
    setLoading(true);
    try {
      const response = await client.getPartnerBoard(trimmedGuild);
      setBoard(response.partner_board);
      applyBoardToForms(response.partner_board);
      setStatus({
        kind: "success",
        message: `Loaded partner board for guild ${trimmedGuild}.`,
      });
    } catch (error) {
      setStatus({
        kind: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
    }
  }

  async function refreshPartnersOnly() {
    if (!ensureGuildSelected()) {
      return;
    }

    const trimmedGuild = guildID.trim();
    setLoading(true);
    try {
      const response = await client.listPartners(trimmedGuild);
      setBoard((prev) => ({
        ...(prev ?? {}),
        partners: response.partners,
      }));
      setStatus({
        kind: "success",
        message: `Loaded ${response.partners.length} partners for guild ${trimmedGuild}.`,
      });
    } catch (error) {
      setStatus({
        kind: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
    }
  }

  async function saveTarget() {
    if (!ensureGuildSelected()) {
      return;
    }

    const payload = buildTargetPayload(targetForm);
    setLoading(true);
    try {
      await client.setPartnerBoardTarget(guildID.trim(), payload);
      await refreshBoard();
      setStatus({
        kind: "success",
        message: "Target updated and board reloaded.",
      });
    } catch (error) {
      setStatus({
        kind: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
    }
  }

  async function saveTemplate() {
    if (!ensureGuildSelected()) {
      return;
    }

    const payload: PartnerBoardTemplateConfig = {
      ...(board?.template ?? {}),
      title: templateForm.title.trim(),
      intro: templateForm.intro,
      section_header_template: templateForm.sectionHeaderTemplate,
      line_template: templateForm.lineTemplate,
      empty_state_text: templateForm.emptyStateText,
    };

    setLoading(true);
    try {
      await client.setPartnerBoardTemplate(guildID.trim(), payload);
      await refreshBoard();
      setStatus({
        kind: "success",
        message: "Template updated and board reloaded.",
      });
    } catch (error) {
      setStatus({
        kind: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
    }
  }

  async function addPartner() {
    if (!ensureGuildSelected()) {
      return;
    }

    setLoading(true);
    try {
      await client.createPartner(guildID.trim(), {
        fandom: partnerForm.fandom,
        name: partnerForm.name,
        link: partnerForm.link,
      });
      setPartnerForm(initialPartnerForm);
      await refreshBoard();
      setStatus({
        kind: "success",
        message: "Partner created.",
      });
    } catch (error) {
      setStatus({
        kind: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
    }
  }

  async function updatePartner() {
    if (!ensureGuildSelected()) {
      return;
    }

    setLoading(true);
    try {
      await client.updatePartner(guildID.trim(), partnerUpdateForm.currentName, {
        fandom: partnerUpdateForm.fandom,
        name: partnerUpdateForm.name,
        link: partnerUpdateForm.link,
      });
      setPartnerUpdateForm(initialPartnerUpdateForm);
      await refreshBoard();
      setStatus({
        kind: "success",
        message: "Partner updated.",
      });
    } catch (error) {
      setStatus({
        kind: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
    }
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

    setLoading(true);
    try {
      await client.deletePartner(guildID.trim(), partnerDeleteName);
      setPartnerDeleteName("");
      await refreshBoard();
      setStatus({
        kind: "success",
        message: "Partner deleted.",
      });
    } catch (error) {
      setStatus({
        kind: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
    }
  }

  async function syncBoard() {
    if (!ensureGuildSelected()) {
      return;
    }

    setLoading(true);
    try {
      await client.syncPartnerBoard(guildID.trim());
      setStatus({
        kind: "success",
        message: "Partner board sync requested successfully.",
      });
    } catch (error) {
      setStatus({
        kind: "error",
        message: formatError(error),
      });
    } finally {
      setLoading(false);
    }
  }

  function ensureGuildSelected(): boolean {
    if (authState !== "signed_in") {
      setStatus({
        kind: "error",
        message: "Sign in with Discord before editing guild settings.",
      });
      return false;
    }
    if (guildID.trim() === "") {
      setStatus({
        kind: "error",
        message: "Select a guild you can manage first.",
      });
      return false;
    }
    return true;
  }

  function resetLoadedBoard() {
    setGuildID("");
    setBoard(null);
    setTargetForm(initialTargetForm);
    setTemplateForm(initialTemplateForm);
    setPartnerForm(initialPartnerForm);
    setPartnerUpdateForm(initialPartnerUpdateForm);
    setPartnerDeleteName("");
  }

  function applyBoardToForms(nextBoard: PartnerBoardConfig) {
    const target = nextBoard.target;
    if (target) {
      setTargetForm({
        type:
          target.type === "webhook_message" ? "webhook_message" : "channel_message",
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

  return (
    <main className="shell">
      <section className="panel">
        <header className="header">
          <p className="eyebrow">Discordcore</p>
          <h1>Partner Board Admin</h1>
          <p className="muted">
            The dashboard stays available on the local control server, but guild
            configuration through the web UI requires a Discord OAuth session.
          </p>
          <div className="theme-switcher" role="tablist" aria-label="Theme selector">
            {themeOptions.map((option) => (
              <button
                key={option.id}
                type="button"
                role="tab"
                aria-selected={theme === option.id}
                className={`theme-option ${theme === option.id ? "is-active" : ""}`}
                onClick={() => setTheme(option.id)}
              >
                <span>{option.label}</span>
                <small>{option.helper}</small>
              </button>
            ))}
          </div>
        </header>

        <section className="card">
          <h2>Connection</h2>
          <div className="grid two">
            <label>
              Base URL
              <input
                value={baseUrl}
                onChange={(event) => setBaseUrl(event.target.value)}
                placeholder="http://127.0.0.1:8376"
              />
            </label>
            <label>
              Guild
              <select
                value={guildID}
                onChange={(event) => {
                  setGuildID(event.target.value);
                  setBoard(null);
                }}
                disabled={authState !== "signed_in" || manageableGuilds.length === 0}
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
          </div>

          <div className="session-card">
            <div>
              <p className="session-label">Auth status</p>
              {authState === "checking" ? (
                <p className="muted">Checking current dashboard session...</p>
              ) : null}
              {authState === "oauth_unavailable" ? (
                <p className="muted">
                  OAuth is not configured on the server. The dashboard shell is
                  available, but web-based guild editing is disabled.
                </p>
              ) : null}
              {authState === "signed_out" ? (
                <p className="muted">
                  Sign in with Discord to access only the guilds you can manage.
                </p>
              ) : null}
              {authState === "signed_in" && session !== null ? (
                <div className="session-details">
                  <p className="session-user">{formatUserLabel(session)}</p>
                  <p className="muted">
                    {manageableGuilds.length} manageable guild
                    {manageableGuilds.length === 1 ? "" : "s"} available.
                  </p>
                </div>
              ) : null}
            </div>

            <div className="actions">
              {authState === "signed_out" ? (
                <button
                  className="button-primary"
                  type="button"
                  disabled={loading}
                  onClick={() => {
                    window.location.assign(client.buildDiscordLoginURL("/dashboard/"));
                  }}
                >
                  Sign In with Discord
                </button>
              ) : null}
              {authState === "signed_in" ? (
                <button
                  className="button-ghost"
                  type="button"
                  disabled={loading}
                  onClick={() => void logout()}
                >
                  Sign Out
                </button>
              ) : null}
              <button
                className="button-secondary"
                type="button"
                disabled={loading}
                onClick={() => void refreshSession()}
              >
                Refresh Session
              </button>
            </div>
          </div>

          <p className="helper">
            Guild selection is limited to servers returned by
            `/auth/guilds/manageable`, intersected with the guilds where the bot is
            present.
          </p>

          <div className="actions">
            <button
              className="button-primary"
              type="button"
              disabled={!canManageGuild}
              onClick={() => void refreshBoard()}
            >
              Load Board
            </button>
            <button
              type="button"
              disabled={!canManageGuild}
              onClick={() => void refreshPartnersOnly()}
            >
              Refresh Partners
            </button>
            <button
              className="button-secondary"
              type="button"
              disabled={!canManageGuild}
              onClick={() => void syncBoard()}
            >
              Sync Board
            </button>
          </div>
        </section>

        <section className="card">
          <h2>Target</h2>
          <div className="grid two">
            <label>
              Type
              <select
                value={targetForm.type}
                onChange={(event) =>
                  setTargetForm((prev) => ({
                    ...prev,
                    type: event.target.value as "webhook_message" | "channel_message",
                  }))
                }
                disabled={authState !== "signed_in"}
              >
                <option value="channel_message">channel_message</option>
                <option value="webhook_message">webhook_message</option>
              </select>
            </label>
            <label>
              message_id
              <input
                value={targetForm.messageID}
                onChange={(event) =>
                  setTargetForm((prev) => ({ ...prev, messageID: event.target.value }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
            <label>
              channel_id
              <input
                value={targetForm.channelID}
                onChange={(event) =>
                  setTargetForm((prev) => ({ ...prev, channelID: event.target.value }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
            <label>
              webhook_url
              <input
                value={targetForm.webhookURL}
                onChange={(event) =>
                  setTargetForm((prev) => ({ ...prev, webhookURL: event.target.value }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
          </div>
          <div className="actions">
            <button
              className="button-primary"
              type="button"
              disabled={!canManageGuild}
              onClick={() => void saveTarget()}
            >
              Save Target
            </button>
          </div>
        </section>

        <section className="card">
          <h2>Template</h2>
          <div className="grid two">
            <label>
              title
              <input
                value={templateForm.title}
                onChange={(event) =>
                  setTemplateForm((prev) => ({ ...prev, title: event.target.value }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
            <label>
              section_header_template
              <input
                value={templateForm.sectionHeaderTemplate}
                onChange={(event) =>
                  setTemplateForm((prev) => ({
                    ...prev,
                    sectionHeaderTemplate: event.target.value,
                  }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
            <label>
              line_template
              <input
                value={templateForm.lineTemplate}
                onChange={(event) =>
                  setTemplateForm((prev) => ({ ...prev, lineTemplate: event.target.value }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
            <label>
              empty_state_text
              <input
                value={templateForm.emptyStateText}
                onChange={(event) =>
                  setTemplateForm((prev) => ({
                    ...prev,
                    emptyStateText: event.target.value,
                  }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
          </div>
          <label>
            intro
            <textarea
              rows={4}
              value={templateForm.intro}
              onChange={(event) =>
                setTemplateForm((prev) => ({ ...prev, intro: event.target.value }))
              }
              disabled={authState !== "signed_in"}
            />
          </label>
          <div className="actions">
            <button
              className="button-primary"
              type="button"
              disabled={!canManageGuild}
              onClick={() => void saveTemplate()}
            >
              Save Template
            </button>
          </div>
        </section>

        <section className="card">
          <h2>Partners</h2>
          <p className="muted">
            Current partners: <strong>{partners.length}</strong>
          </p>
          <table>
            <thead>
              <tr>
                <th>Fandom</th>
                <th>Name</th>
                <th>Link</th>
              </tr>
            </thead>
            <tbody>
              {partners.length === 0 ? (
                <tr>
                  <td colSpan={3}>No partners configured.</td>
                </tr>
              ) : (
                partners.map((partner) => (
                  <tr key={`${partner.name}|${partner.link}`}>
                    <td>{partner.fandom ?? ""}</td>
                    <td>{partner.name}</td>
                    <td>{partner.link}</td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </section>

        <section className="card">
          <h2>Add Partner</h2>
          <div className="grid three">
            <label>
              fandom
              <input
                value={partnerForm.fandom}
                onChange={(event) =>
                  setPartnerForm((prev) => ({ ...prev, fandom: event.target.value }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
            <label>
              name
              <input
                value={partnerForm.name}
                onChange={(event) =>
                  setPartnerForm((prev) => ({ ...prev, name: event.target.value }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
            <label>
              link
              <input
                value={partnerForm.link}
                onChange={(event) =>
                  setPartnerForm((prev) => ({ ...prev, link: event.target.value }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
          </div>
          <div className="actions">
            <button
              className="button-primary"
              type="button"
              disabled={!canManageGuild}
              onClick={() => void addPartner()}
            >
              Add Partner
            </button>
          </div>
        </section>

        <section className="card">
          <h2>Update Partner</h2>
          <div className="grid two">
            <label>
              current_name
              <input
                value={partnerUpdateForm.currentName}
                onChange={(event) =>
                  setPartnerUpdateForm((prev) => ({
                    ...prev,
                    currentName: event.target.value,
                  }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
            <label>
              fandom
              <input
                value={partnerUpdateForm.fandom}
                onChange={(event) =>
                  setPartnerUpdateForm((prev) => ({ ...prev, fandom: event.target.value }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
            <label>
              name
              <input
                value={partnerUpdateForm.name}
                onChange={(event) =>
                  setPartnerUpdateForm((prev) => ({ ...prev, name: event.target.value }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
            <label>
              link
              <input
                value={partnerUpdateForm.link}
                onChange={(event) =>
                  setPartnerUpdateForm((prev) => ({ ...prev, link: event.target.value }))
                }
                disabled={authState !== "signed_in"}
              />
            </label>
          </div>
          <div className="actions">
            <button
              className="button-primary"
              type="button"
              disabled={!canManageGuild}
              onClick={() => void updatePartner()}
            >
              Update Partner
            </button>
          </div>
        </section>

        <section className="card">
          <h2>Delete Partner</h2>
          <div className="grid one">
            <label>
              name
              <input
                value={partnerDeleteName}
                onChange={(event) => setPartnerDeleteName(event.target.value)}
                disabled={authState !== "signed_in"}
              />
            </label>
          </div>
          <div className="actions">
            <button
              className="button-danger"
              type="button"
              disabled={!canManageGuild}
              onClick={() => void deletePartner()}
            >
              Delete Partner
            </button>
          </div>
        </section>

        <footer className={`status status-${status.kind}`}>{status.message}</footer>
      </section>
    </main>
  );
}

function formatError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

function formatUserLabel(session: AuthSessionResponse): string {
  const displayName =
    session.user.global_name?.trim() || session.user.username.trim() || session.user.id;
  return `${displayName} (${session.user.id})`;
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

function resolveInitialTheme(): DashboardTheme {
  const storedTheme = window.localStorage.getItem(themeStorageKey);
  if (
    storedTheme === "investigadora-paranormal" ||
    storedTheme === "forum-spook-shack"
  ) {
    return storedTheme;
  }
  return themeOptions[0].id;
}
