import React from "react";
import type { DiscordMessage } from "./types";
import { TranscriptEmbed } from "./TranscriptEmbed";
import "./transcript.css";

interface Props {
  msg: DiscordMessage;
  isSameAuthor: boolean;
}

export const TranscriptMessage: React.FC<Props> = ({ msg, isSameAuthor }) => {
  return (
    <div className={`transcript-message ${isSameAuthor ? "grouped" : ""}`}>
      {!isSameAuthor && (
        <div className="transcript-avatar">
          <img 
            src={msg.author.avatar ? `https://cdn.discordapp.com/avatars/${msg.author.id}/${msg.author.avatar}.png` : "https://cdn.discordapp.com/embed/avatars/0.png"} 
            alt={msg.author.username} 
          />
        </div>
      )}
      <div className="transcript-content-wrapper">
        {!isSameAuthor && (
          <div className="transcript-header">
            <span className="transcript-username">{msg.author.username}</span>
            <span className="transcript-timestamp">{new Date(msg.timestamp).toLocaleString()}</span>
          </div>
        )}
        {msg.content && <div className="transcript-content">{msg.content}</div>}
        
        {/* Embeds */}
        {msg.embeds && msg.embeds.map((embed, i) => (
          <TranscriptEmbed key={i} embed={embed} />
        ))}

        {/* Components */}
        {msg.components && msg.components.map((row, i) => (
          <div key={i} className="transcript-action-row">
            {row.components && row.components.map((btn, j) => (
              <button key={j} className={`transcript-btn style-${btn.style}`} disabled>
                {btn.label}
              </button>
            ))}
          </div>
        ))}
      </div>
    </div>
  );
};
