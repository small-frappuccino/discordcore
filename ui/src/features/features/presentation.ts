import type { FeatureRecord } from "../../api/control";
import type { FeatureWorkspaceState } from "./useFeatureWorkspace";
import {
  getFeatureBlockerSummary,
  isFeatureBlocked,
  isFeatureConfigurable,
} from "./model";

export type FeatureStatusTone = "neutral" | "info" | "success" | "error";

export interface FeatureAreaSummary {
  total: number;
  ready: number;
  blocked: number;
  disabled: number;
  label: string;
  signal: string;
  tone: FeatureStatusTone;
}

export function summarizeFeatureArea(
  features: FeatureRecord[],
): FeatureAreaSummary {
  const total = features.length;
  const ready = features.filter((feature) => feature.readiness === "ready").length;
  const blocked = features.filter((feature) => feature.readiness === "blocked").length;
  const disabled = features.filter((feature) => !feature.effective_enabled).length;

  if (total === 0) {
    return {
      total,
      ready,
      blocked,
      disabled,
      label: "No records",
      signal: "No feature records are mapped to this category yet.",
      tone: "info",
    };
  }

  if (blocked > 0) {
    return {
      total,
      ready,
      blocked,
      disabled,
      label: "Needs attention",
      signal:
        features.find((feature) => feature.blockers?.length)?.blockers?.[0]
          ?.message ??
        "One or more features in this category are blocked.",
      tone: "error",
    };
  }

  if (ready > 0) {
    return {
      total,
      ready,
      blocked,
      disabled,
      label: "Operational",
      signal:
        disabled === 0
          ? "Every mapped feature is currently ready."
          : "At least one feature is ready while the rest stay disabled.",
      tone: "success",
    };
  }

  return {
    total,
    ready,
    blocked,
    disabled,
    label: "Disabled",
    signal: "Every mapped feature is currently disabled.",
    tone: "neutral",
  };
}

export function formatFeatureStatusLabel(feature: FeatureRecord) {
  if (!feature.effective_enabled) {
    return "Disabled";
  }

  if (feature.readiness === "ready") {
    return "Ready";
  }

  if (feature.readiness === "blocked") {
    return "Needs attention";
  }

  return "Checking";
}

export function getFeatureStatusTone(
  feature: FeatureRecord,
): FeatureStatusTone {
  if (!feature.effective_enabled) {
    return "neutral";
  }

  if (feature.readiness === "ready") {
    return "success";
  }

  if (feature.readiness === "blocked") {
    return "error";
  }

  return "info";
}

export function formatFeatureStatusSupport(feature: FeatureRecord) {
  if (!feature.effective_enabled) {
    return "This feature is currently turned off for the selected server.";
  }

  if (feature.readiness === "ready") {
    return "No blockers are currently reported by the control server.";
  }

  if (feature.readiness === "blocked") {
    return "The feature is enabled but still needs setup or another dependency.";
  }

  return "The control server is still checking this feature.";
}

export function formatOverrideLabel(overrideState: string) {
  switch (overrideState) {
    case "enabled":
      return "Enabled for this server";
    case "disabled":
      return "Disabled for this server";
    case "inherit":
      return "Using inherited setting";
    case "default":
      return "Using default setting";
    default:
      return "Using configured setting";
  }
}

export function formatEffectiveSourceLabel(source: string) {
  switch (source) {
    case "guild":
      return "Applied from the selected server.";
    case "global":
      return "Inherited from global dashboard settings.";
    case "built_in":
      return "Inherited from built-in defaults.";
    default:
      return "Applied from the current dashboard configuration.";
  }
}

export function formatFeatureSignalTitle(feature: FeatureRecord) {
  if (isFeatureBlocked(feature)) {
    return "Blocker";
  }

  if (isFeatureConfigurable(feature)) {
    return "Additional settings";
  }

  if (!feature.effective_enabled) {
    return "State";
  }

  return "Signal";
}

export function formatFeatureSignal(feature: FeatureRecord) {
  if (isFeatureBlocked(feature)) {
    return getFeatureBlockerSummary(feature);
  }

  if (isFeatureConfigurable(feature)) {
    return "This feature exposes extra settings beyond the enabled state.";
  }

  if (!feature.effective_enabled) {
    return "The feature is off, so no readiness signal is expected.";
  }

  return "The control server is not reporting blockers for this feature.";
}

export function formatWorkspaceStateTitle(
  areaLabel: string,
  state: FeatureWorkspaceState,
) {
  switch (state) {
    case "checking":
      return "Checking access";
    case "auth_required":
      return "Sign in required";
    case "server_required":
      return "Select a server";
    case "loading":
      return `Loading ${areaLabel.toLowerCase()}`;
    case "unavailable":
      return `${areaLabel} unavailable`;
    case "ready":
      return `Manage ${areaLabel.toLowerCase()}`;
    default:
      return `${areaLabel} unavailable`;
  }
}

export function formatWorkspaceStateDescription(
  areaLabel: string,
  state: FeatureWorkspaceState,
) {
  switch (state) {
    case "checking":
      return "The dashboard is checking the current session before loading this category.";
    case "auth_required":
      return `Sign in with Discord to load the ${areaLabel.toLowerCase()} workspace.`;
    case "server_required":
      return "Choose a server from the sidebar before loading category settings.";
    case "loading":
      return `The latest ${areaLabel.toLowerCase()} feature records are loading for the selected server.`;
    case "unavailable":
      return `The ${areaLabel.toLowerCase()} workspace could not be loaded for the selected server.`;
    case "ready":
      return `Manage ${areaLabel.toLowerCase()} for the selected server.`;
    default:
      return `The ${areaLabel.toLowerCase()} workspace is unavailable.`;
  }
}
