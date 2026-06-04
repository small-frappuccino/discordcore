import React from "react";
import type { DiscordEmbed } from "./types";
import "./transcript.css";

interface Props {
  embed: DiscordEmbed;
}

export const TranscriptEmbed: React.FC<Props> = ({ embed }) => {
  const borderColor = embed.color 
    ? `#${embed.color.toString(16).padStart(6, '0')}` 
    : '#202225';

  return (
    <div className="transcript-embed" style={{ borderLeftColor: borderColor }}>
      {embed.title && <div className="transcript-embed-title">{embed.title}</div>}
      {embed.description && <div className="transcript-embed-description">{embed.description}</div>}
    </div>
  );
};
