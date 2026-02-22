import { useMemo, useState } from "react";
import {
  type EmbedUpdateTargetConfig,
  type PartnerBoardConfig,
  type PartnerBoardTemplateConfig,
  ControlApiClient,
} from "./api/control";

type StatusKind = "idle" | "success" | "error" | "info";

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
const defaultBearerToken = import.meta.env.VITE_CONTROL_API_BEARER_TOKEN ?? "";
const defaultGuildID = import.meta.env.VITE_CONTROL_API_GUILD_ID ?? "";

export default function App() {
  const [baseUrl, setBaseUrl] = useState(defaultBaseUrl);
  const [bearerToken, setBearerToken] = useState(defaultBearerToken);
  const [guildID, setGuildID] = useState(defaultGuildID);
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

  const client = useMemo(
    () =>
      new ControlApiClient({
        baseUrl,
        bearerToken,
      }),
    [baseUrl, bearerToken],
  );

  async function refreshBoard() {
    const trimmedGuild = guildID.trim();
    if (trimmedGuild === "") {
      setStatus({ kind: "error", message: "guild_id is required." });
      return;
    }

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
    const trimmedGuild = guildID.trim();
    if (trimmedGuild === "") {
      setStatus({ kind: "error", message: "guild_id is required." });
      return;
    }

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
    const trimmedGuild = guildID.trim();
    if (trimmedGuild === "") {
      setStatus({ kind: "error", message: "guild_id is required." });
      return;
    }

    const payload = buildTargetPayload(targetForm);
    setLoading(true);
    try {
      await client.setPartnerBoardTarget(trimmedGuild, payload);
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
    const trimmedGuild = guildID.trim();
    if (trimmedGuild === "") {
      setStatus({ kind: "error", message: "guild_id is required." });
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
      await client.setPartnerBoardTemplate(trimmedGuild, payload);
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
    const trimmedGuild = guildID.trim();
    if (trimmedGuild === "") {
      setStatus({ kind: "error", message: "guild_id is required." });
      return;
    }

    setLoading(true);
    try {
      await client.createPartner(trimmedGuild, {
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
    const trimmedGuild = guildID.trim();
    if (trimmedGuild === "") {
      setStatus({ kind: "error", message: "guild_id is required." });
      return;
    }

    setLoading(true);
    try {
      await client.updatePartner(trimmedGuild, partnerUpdateForm.currentName, {
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
    const trimmedGuild = guildID.trim();
    if (trimmedGuild === "") {
      setStatus({ kind: "error", message: "guild_id is required." });
      return;
    }
    if (partnerDeleteName.trim() === "") {
      setStatus({ kind: "error", message: "Partner name to delete is required." });
      return;
    }

    setLoading(true);
    try {
      await client.deletePartner(trimmedGuild, partnerDeleteName);
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
    const trimmedGuild = guildID.trim();
    if (trimmedGuild === "") {
      setStatus({ kind: "error", message: "guild_id is required." });
      return;
    }

    setLoading(true);
    try {
      await client.syncPartnerBoard(trimmedGuild);
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

  const partners = board?.partners ?? [];

  return (
    <main className="shell">
      <section className="panel">
        <header className="header">
          <p className="eyebrow">Discordcore</p>
          <h1>Partner Board Admin</h1>
          <p className="muted">
            Control API wiring for partner board target/template/partners and sync.
          </p>
        </header>

        <section className="card">
          <h2>Connection</h2>
          <p className="muted">
            Use current origin to leverage Vite proxy for `/v1` requests.
          </p>
          <div className="grid two">
            <label>
              Base URL
              <input
                value={baseUrl}
                onChange={(event) => setBaseUrl(event.target.value)}
                placeholder="http://127.0.0.1:8080"
              />
            </label>
            <label>
              Guild ID
              <input
                value={guildID}
                onChange={(event) => setGuildID(event.target.value)}
                placeholder="123456789012345678"
              />
            </label>
          </div>
          <label>
            Bearer token
            <input
              value={bearerToken}
              onChange={(event) => setBearerToken(event.target.value)}
              placeholder="ALICE_CONTROL_BEARER_TOKEN"
              type="password"
            />
          </label>
          <div className="actions">
            <button disabled={loading} onClick={() => void refreshBoard()}>
              Load Board
            </button>
            <button disabled={loading} onClick={() => void refreshPartnersOnly()}>
              Refresh Partners
            </button>
            <button disabled={loading} onClick={() => void syncBoard()}>
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
              />
            </label>
            <label>
              channel_id
              <input
                value={targetForm.channelID}
                onChange={(event) =>
                  setTargetForm((prev) => ({ ...prev, channelID: event.target.value }))
                }
              />
            </label>
            <label>
              webhook_url
              <input
                value={targetForm.webhookURL}
                onChange={(event) =>
                  setTargetForm((prev) => ({ ...prev, webhookURL: event.target.value }))
                }
              />
            </label>
          </div>
          <div className="actions">
            <button disabled={loading} onClick={() => void saveTarget()}>
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
              />
            </label>
            <label>
              line_template
              <input
                value={templateForm.lineTemplate}
                onChange={(event) =>
                  setTemplateForm((prev) => ({ ...prev, lineTemplate: event.target.value }))
                }
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
            />
          </label>
          <div className="actions">
            <button disabled={loading} onClick={() => void saveTemplate()}>
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
              />
            </label>
            <label>
              name
              <input
                value={partnerForm.name}
                onChange={(event) =>
                  setPartnerForm((prev) => ({ ...prev, name: event.target.value }))
                }
              />
            </label>
            <label>
              link
              <input
                value={partnerForm.link}
                onChange={(event) =>
                  setPartnerForm((prev) => ({ ...prev, link: event.target.value }))
                }
              />
            </label>
          </div>
          <div className="actions">
            <button disabled={loading} onClick={() => void addPartner()}>
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
              />
            </label>
            <label>
              fandom
              <input
                value={partnerUpdateForm.fandom}
                onChange={(event) =>
                  setPartnerUpdateForm((prev) => ({ ...prev, fandom: event.target.value }))
                }
              />
            </label>
            <label>
              name
              <input
                value={partnerUpdateForm.name}
                onChange={(event) =>
                  setPartnerUpdateForm((prev) => ({ ...prev, name: event.target.value }))
                }
              />
            </label>
            <label>
              link
              <input
                value={partnerUpdateForm.link}
                onChange={(event) =>
                  setPartnerUpdateForm((prev) => ({ ...prev, link: event.target.value }))
                }
              />
            </label>
          </div>
          <div className="actions">
            <button disabled={loading} onClick={() => void updatePartner()}>
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
              />
            </label>
          </div>
          <div className="actions">
            <button disabled={loading} onClick={() => void deletePartner()}>
              Delete Partner
            </button>
          </div>
        </section>

        <footer className={`status status-${status.kind}`}>{status.message}</footer>
      </section>
    </main>
  );

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
}

function formatError(error: unknown): string {
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
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
