import React, { useEffect, useState } from "react";
import type { DiscordMessage } from "./types";
import { TranscriptMessage } from "./TranscriptMessage";
import "./transcript.css";

export const TranscriptViewer: React.FC = () => {
  const [messages, setMessages] = useState<DiscordMessage[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const url = params.get("url");
    if (!url) {
      setError("No transcript URL provided.");
      return;
    }

    fetch(url)
      .then((res) => {
        if (!res.ok) throw new Error("Failed to fetch transcript.");
        return res.json();
      })
      .then((data) => setMessages(data))
      .catch((err) => setError(err.message));
  }, []);

  if (error) {
    return <div className="transcript-error">{error}</div>;
  }

  if (!messages) {
    return <div className="transcript-loading">Loading transcript...</div>;
  }

  return (
    <div className="transcript-container">
      {messages.map((msg, index) => {
        const prevMsg = index > 0 ? messages[index - 1] : null;
        const isSameAuthor = Boolean(prevMsg && prevMsg.author.id === msg.author.id);
        
        return (
          <TranscriptMessage 
            key={msg.id} 
            msg={msg} 
            isSameAuthor={isSameAuthor} 
          />
        );
      })}
    </div>
  );
};
