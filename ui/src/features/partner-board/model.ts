import type {
  EmbedUpdateTargetConfig,
  PartnerBoardConfig,
  PartnerBoardTemplateConfig,
} from "../../api/control";
import type { DashboardAuthState } from "../../app/types";

export interface DeliveryFormState {
  type: "webhook_message" | "channel_message";
  messageID: string;
  webhookURL: string;
  channelID: string;
}

export interface LayoutFormState {
  title: string;
  intro: string;
  sectionHeaderTemplate: string;
  lineTemplate: string;
  emptyStateText: string;
}

export interface EntryFormState {
  fandom: string;
  name: string;
  link: string;
}

export interface PartnerBoardShellStatus {
  description: string;
  label: string;
  tone: "neutral" | "info" | "success" | "error";
}

export interface DeliveryChecklistItem {
  complete: boolean;
  label: string;
}

export const initialDeliveryForm: DeliveryFormState = {
  type: "channel_message",
  messageID: "",
  webhookURL: "",
  channelID: "",
};

export const initialLayoutForm: LayoutFormState = {
  title: "",
  intro: "",
  sectionHeaderTemplate: "",
  lineTemplate: "",
  emptyStateText: "",
};

export const initialEntryForm: EntryFormState = {
  fandom: "",
  name: "",
  link: "",
};

export function formsFromBoard(
  board: PartnerBoardConfig | null,
): {
  deliveryForm: DeliveryFormState;
  layoutForm: LayoutFormState;
} {
  if (board === null) {
    return {
      deliveryForm: initialDeliveryForm,
      layoutForm: initialLayoutForm,
    };
  }

  return {
    deliveryForm: {
      type:
        board.target?.type === "webhook_message"
          ? "webhook_message"
          : "channel_message",
      messageID: board.target?.message_id ?? "",
      webhookURL: board.target?.webhook_url ?? "",
      channelID: board.target?.channel_id ?? "",
    },
    layoutForm: {
      title: board.template?.title ?? "",
      intro: board.template?.intro ?? "",
      sectionHeaderTemplate: board.template?.section_header_template ?? "",
      lineTemplate: board.template?.line_template ?? "",
      emptyStateText: board.template?.empty_state_text ?? "",
    },
  };
}

export function buildDeliveryPayload(
  form: DeliveryFormState,
): EmbedUpdateTargetConfig {
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

export function buildLayoutPayload(
  form: LayoutFormState,
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

export function isDeliveryConfigured(target?: EmbedUpdateTargetConfig): boolean {
  if (target === undefined || target.message_id?.trim() === "") {
    return false;
  }

  if (target.type === "webhook_message") {
    return (target.webhook_url?.trim() ?? "") !== "";
  }

  return (target.channel_id?.trim() ?? "") !== "";
}

export function isLayoutConfigured(
  template?: PartnerBoardTemplateConfig,
): boolean {
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

export function validateDeliveryForm(form: DeliveryFormState): string | null {
  if (form.messageID.trim() === "") {
    return "Board message ID is required before saving the posting destination.";
  }

  if (form.type === "webhook_message" && form.webhookURL.trim() === "") {
    return "Webhook URL is required for webhook posting.";
  }

  if (form.type === "channel_message" && form.channelID.trim() === "") {
    return "Channel ID is required for channel posting.";
  }

  return null;
}

export function getDeliveryChecklist(
  form: DeliveryFormState,
): DeliveryChecklistItem[] {
  const usesWebhook = form.type === "webhook_message";

  return [
    {
      complete: form.messageID.trim() !== "",
      label: "Board message target is saved",
    },
    usesWebhook
      ? {
          complete: form.webhookURL.trim() !== "",
          label: "Webhook connection is set",
        }
      : {
          complete: form.channelID.trim() !== "",
          label: "Channel is selected",
        },
  ];
}

export function getDeliveryGuidance(
  form: DeliveryFormState,
  configured: boolean,
): string {
  if (configured) {
    return form.type === "webhook_message"
      ? "This board is ready to publish through a webhook destination."
      : "This board is ready to publish to a Discord channel.";
  }

  return form.type === "webhook_message"
    ? "Finish the webhook connection in Diagnostics before relying on this board."
    : "Finish the channel destination in Diagnostics before relying on this board.";
}

export function validateEntryForm(form: EntryFormState): string | null {
  if (form.name.trim() === "") {
    return "Partner name is required.";
  }

  if (form.link.trim() === "") {
    return "Invite link is required.";
  }

  return null;
}

export function postingMethodLabel(
  value: DeliveryFormState["type"] | EmbedUpdateTargetConfig["type"] | "",
) {
  if (value === "webhook_message") {
    return "Webhook message";
  }
  if (value === "channel_message") {
    return "Channel message";
  }
  return "Not set";
}

export function summarizePostingDestination(target?: EmbedUpdateTargetConfig) {
  if (target === undefined) {
    return "Not set";
  }

  if (!isDeliveryConfigured(target)) {
    return target.type === "webhook_message"
      ? "Webhook details missing"
      : "Channel details missing";
  }

  return postingMethodLabel(target.type ?? "");
}

export function countFilledLayoutFields(
  template?: PartnerBoardTemplateConfig,
) {
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

export function getPartnerBoardShellStatus(input: {
  authState: DashboardAuthState;
  board: PartnerBoardConfig | null;
  deliveryConfigured: boolean;
  hasLoadedAttempt: boolean;
  lastSyncedAt: number | null;
  layoutConfigured: boolean;
  loading: boolean;
  partnerCount: number;
  selectedGuildID: string;
}): PartnerBoardShellStatus {
  if (input.authState === "checking") {
    return {
      description: "Checking dashboard access.",
      label: "Checking access",
      tone: "info",
    };
  }

  if (input.authState !== "signed_in") {
    return {
      description: "Sign in with Discord to manage partner boards.",
      label: "Sign in required",
      tone: "info",
    };
  }

  if (input.selectedGuildID.trim() === "") {
    return {
      description: "Choose a server from the sidebar to load its board settings.",
      label: "Choose a server",
      tone: "info",
    };
  }

  if (input.loading && input.board === null) {
    return {
      description: "Loading the latest board settings for this server.",
      label: "Loading board",
      tone: "info",
    };
  }

  if (input.board === null && input.hasLoadedAttempt) {
    return {
      description: "The dashboard could not load this server's Partner Board configuration.",
      label: "Unavailable",
      tone: "error",
    };
  }

  if (
    !input.deliveryConfigured ||
    !input.layoutConfigured ||
    input.partnerCount === 0
  ) {
    return {
      description: "Finish the destination, layout, and first partner entry before relying on this board.",
      label: "Needs setup",
      tone: "info",
    };
  }

  if (input.lastSyncedAt !== null) {
    return {
      description: "The current session has synced this board to Discord.",
      label: "Synced",
      tone: "success",
    };
  }

  return {
    description: "The board configuration is loaded and ready to manage.",
    label: "Configured",
    tone: "success",
  };
}
