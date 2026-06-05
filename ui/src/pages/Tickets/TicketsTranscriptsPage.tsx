import { useState } from "react";
import { useCurrentGuild } from "../../context/GuildContext";
import { useLiveTickets, useTranscriptsList } from "../../api/hooks/useTickets";
import type { LiveTicket, ClosedTicketTranscript } from "../../api/domains/tickets";
import { Badge, SurfaceCard, Button } from "../../components/ui";
import { TranscriptViewer } from "./components/TranscriptViewer";

export function TicketsTranscriptsPage() {
  const { guildId: selectedGuildID } = useCurrentGuild();
  const { data: liveResp, isLoading: isLoadingLive } = useLiveTickets(selectedGuildID || "");
  const { data: transResp, isLoading: isLoadingTranscripts } = useTranscriptsList(selectedGuildID || "");

  const [activeTab, setActiveTab] = useState<"live" | "closed">("live");
  const [viewingTranscriptId, setViewingTranscriptId] = useState<string | null>(null);

  if (!selectedGuildID) return null;

  if (viewingTranscriptId) {
    return (
      <div className="space-y-4">
        <Button variant="secondary" onClick={() => setViewingTranscriptId(null)}>
          &larr; Back to Transcripts
        </Button>
        <TranscriptViewer guildId={selectedGuildID} transcriptId={viewingTranscriptId} />
      </div>
    );
  }

  return (
    <div>
      <div className="mb-6">
        <h2 className="text-xl font-semibold">Operations & Transcripts</h2>
        <p className="text-muted">Monitor live tickets or review closed historical transcripts.</p>
      </div>

      <div className="flex gap-4 mb-6 border-b border-surface-border pb-4">
        <Button
          variant={activeTab === "live" ? "primary" : "secondary"}
          onClick={() => setActiveTab("live")}
        >
          Live Tickets
        </Button>
        <Button
          variant={activeTab === "closed" ? "primary" : "secondary"}
          onClick={() => setActiveTab("closed")}
        >
          Closed Transcripts
        </Button>
      </div>

      {activeTab === "live" && (
        <div className="space-y-4">
          {isLoadingLive ? (
            <div className="text-muted animate-pulse">Loading live tickets...</div>
          ) : liveResp?.tickets?.length ? (
            liveResp.tickets.map((t: LiveTicket) => (
              <SurfaceCard key={t.id} className="p-4 flex flex-wrap justify-between items-center gap-4">
                <div>
                  <h4 className="font-semibold flex items-center gap-2">
                    Ticket <span className="text-muted">#{t.id}</span>
                  </h4>
                  <p className="text-sm text-muted">
                    Opened by {t.creator_username} • {new Date(t.created_at).toLocaleString()}
                  </p>
                </div>
                <div className="flex items-center gap-4">
                  {t.claimed_by_id ? (
                    <Badge variant="success">Claimed by {t.claimed_by_username}</Badge>
                  ) : (
                    <Badge variant="warning">Unclaimed</Badge>
                  )}
                  <a
                    href={`https://discord.com/channels/${selectedGuildID}/${t.channel_id}`}
                    target="_blank"
                    rel="noreferrer"
                    className="text-primary hover:underline text-sm font-medium"
                  >
                    Jump to Channel &#8599;
                  </a>
                </div>
              </SurfaceCard>
            ))
          ) : (
            <div className="text-center p-8 border-2 border-dashed border-surface-border rounded-xl">
              <p className="text-muted">No active tickets right now.</p>
            </div>
          )}
        </div>
      )}

      {activeTab === "closed" && (
        <div className="space-y-4">
          {isLoadingTranscripts ? (
            <div className="text-muted animate-pulse">Loading transcripts...</div>
          ) : transResp?.transcripts?.length ? (
            transResp.transcripts.map((t: Omit<ClosedTicketTranscript, "messages">) => (
              <SurfaceCard key={t.id} className="p-4 flex flex-wrap justify-between items-center gap-4">
                <div>
                  <h4 className="font-semibold flex items-center gap-2">
                    Ticket <span className="text-muted">#{t.ticket_id}</span>
                  </h4>
                  <p className="text-sm text-muted">
                    Closed on {new Date(t.closed_at).toLocaleDateString()} by {t.closed_by_username}
                  </p>
                  <p className="text-xs text-muted mt-1">
                    Creator: {t.creator_username} • Panel: {t.panel_name}
                  </p>
                </div>
                <Button variant="secondary" onClick={() => setViewingTranscriptId(t.id)}>
                  View Transcript
                </Button>
              </SurfaceCard>
            ))
          ) : (
            <div className="text-center p-8 border-2 border-dashed border-surface-border rounded-xl">
              <p className="text-muted">No closed transcripts found.</p>
            </div>
          )}
        </div>
      )}
    </div>
  );
}
