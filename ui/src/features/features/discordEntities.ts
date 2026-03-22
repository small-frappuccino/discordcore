import type { GuildChannelOption } from "../../api/control";
import type { EntityPickerOption } from "../../components/ui";

export function buildMessageRouteChannelPickerOptions(
  channels: GuildChannelOption[],
): EntityPickerOption[] {
  return channels
    .filter((channel) => channel.supports_message_route)
    .map((channel) => ({
      value: channel.id,
      label: formatGuildChannelOptionLabel(channel),
    }));
}

export function formatGuildChannelValue(
  channelId: string,
  channels: GuildChannelOption[],
  emptyLabel = "Not configured",
) {
  const normalizedChannelId = channelId.trim();
  if (normalizedChannelId === "") {
    return emptyLabel;
  }

  const matchingChannel = channels.find(
    (channel) => channel.id === normalizedChannelId,
  );
  if (matchingChannel !== undefined) {
    return formatGuildChannelOptionLabel(matchingChannel);
  }

  return normalizedChannelId;
}

export function formatGuildChannelOptionLabel(channel: GuildChannelOption) {
  const displayName = channel.display_name?.trim() ?? "";
  if (displayName !== "") {
    return displayName;
  }

  const name = channel.name.trim();
  if (name === "") {
    return channel.id;
  }

  return channel.kind === "text" || channel.kind === "announcement"
    ? `#${name}`
    : name;
}
