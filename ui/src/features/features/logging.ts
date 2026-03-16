import type { FeatureRecord } from "../../api/control";

interface LoggingFeatureDetails {
  requiresChannel: boolean;
  channelId: string;
  validatesChannelPermissions: boolean;
  exclusiveModerationChannel: boolean;
  requiredIntentsMask: number;
  requiredPermissionsMask: number;
  hasRuntimeToggle: boolean;
}

export function getLoggingFeatureDetails(
  feature: FeatureRecord,
): LoggingFeatureDetails {
  const details = feature.details ?? {};

  return {
    requiresChannel: readBoolean(details.requires_channel),
    channelId: readString(details.channel_id),
    validatesChannelPermissions: readBoolean(
      details.validate_channel_permissions,
    ),
    exclusiveModerationChannel: readBoolean(
      details.exclusive_moderation_channel,
    ),
    requiredIntentsMask: readNumber(details.required_intents_mask),
    requiredPermissionsMask: readNumber(details.required_permissions_mask),
    hasRuntimeToggle: readString(details.runtime_toggle_path) !== "",
  };
}

export function canEditLoggingChannel(feature: FeatureRecord) {
  return feature.editable_fields?.includes("channel_id") ?? false;
}

export function summarizeLoggingDestination(feature: FeatureRecord) {
  const details = getLoggingFeatureDetails(feature);

  if (!details.requiresChannel) {
    return "No dedicated destination";
  }

  if (details.channelId !== "") {
    return details.channelId;
  }

  return "Not configured";
}

export function describeLoggingDestination(feature: FeatureRecord) {
  const details = getLoggingFeatureDetails(feature);

  if (!details.requiresChannel) {
    return "This event is tracked without a dedicated destination channel.";
  }

  if (details.channelId !== "") {
    return "Logs currently route to the configured destination channel.";
  }

  return "Choose a destination channel before this event can emit logs.";
}

export function summarizeLoggingGuidance(feature: FeatureRecord) {
  const blockerCode = feature.blockers?.[0]?.code ?? "";

  switch (blockerCode) {
    case "missing_channel":
      return "Choose a destination channel for this logging route.";
    case "invalid_channel":
      return "Choose a different channel. The current destination does not pass backend validation.";
    case "runtime_kill_switch":
      return "A runtime logging switch currently overrides this feature.";
    case "missing_intent":
      return "The runtime host is missing required gateway intents for this event.";
    default:
      return feature.blockers?.[0]?.message ?? describeLoggingDestination(feature);
  }
}

export function buildLoggingRequirementNotes(feature: FeatureRecord) {
  const details = getLoggingFeatureDetails(feature);
  const notes: string[] = [];

  if (details.requiresChannel) {
    notes.push("This event needs an explicit destination channel.");
  } else {
    notes.push("This event does not require a dedicated destination channel.");
  }

  if (details.validatesChannelPermissions) {
    notes.push("The control server validates the destination channel before logs are emitted.");
  }

  if (details.exclusiveModerationChannel) {
    notes.push("This event requires an exclusive moderation channel instead of a shared destination.");
  }

  if (details.requiredIntentsMask !== 0) {
    notes.push("The runtime host needs gateway intents for this event type.");
  }

  if (details.requiredPermissionsMask !== 0) {
    notes.push("The destination channel must satisfy additional Discord permission checks.");
  }

  if (details.hasRuntimeToggle) {
    notes.push("A runtime logging switch can disable this event even when the feature stays enabled.");
  }

  return notes;
}

function readBoolean(value: unknown) {
  return typeof value === "boolean" ? value : false;
}

function readString(value: unknown) {
  return typeof value === "string" ? value.trim() : "";
}

function readNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value) ? value : 0;
}
