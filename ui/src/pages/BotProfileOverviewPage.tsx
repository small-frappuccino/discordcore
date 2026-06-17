import React from "react";
import { useParams } from "react-router-dom";
import { PageHeader } from "../components/ui";

export const BotProfileOverviewPage: React.FC = () => {
  const { guildId, botInstanceId } = useParams<{ guildId: string, botInstanceId: string }>();

  return (
    <div className="surface-card">
      <PageHeader>
        <PageHeader.TitleRow>
          <PageHeader.Title>Bot Profile: {botInstanceId}</PageHeader.Title>
        </PageHeader.TitleRow>
        <PageHeader.Description>Overview for bot instance {botInstanceId} in guild {guildId}</PageHeader.Description>
      </PageHeader>
      <div className="surface-content">
        <p>This is the dedicated dashboard for this bot profile.</p>
        <p>Select a feature from the sidebar to manage it.</p>
      </div>
    </div>
  );
};
