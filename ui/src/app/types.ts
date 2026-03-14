import type { EmbedUpdateTargetConfig } from "../api/control";

export type DashboardAuthState =
  | "checking"
  | "signed_out"
  | "signed_in"
  | "oauth_unavailable";

export type NoticeTone = "info" | "success" | "error";

export interface Notice {
  tone: NoticeTone;
  message: string;
}

export interface SettingsNavigationState {
  diagnostics?: {
    partnerBoardTargetType?: EmbedUpdateTargetConfig["type"];
  };
}
