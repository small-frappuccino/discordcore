import { useTranscriptDetail } from "../../../api/hooks/useTickets";
import { Badge } from "../../../components/ui";

function formatTimestamp(ts: string) {
  const date = new Date(ts);
  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "numeric",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit",
  });
}

export function TranscriptViewer({ guildId, transcriptId }: { guildId: string; transcriptId: string }) {
  const { data, isLoading, error } = useTranscriptDetail(guildId, transcriptId);

  if (isLoading) {
    return <div className="p-8 text-center text-muted animate-pulse">Loading transcript details...</div>;
  }

  if (error || !data) {
    return <div className="p-8 text-center text-red-500">Failed to load transcript.</div>;
  }

  const { transcript } = data;

  return (
    <div className="flex flex-col h-full max-h-[800px] bg-[#313338] text-white rounded-lg overflow-hidden border border-surface-border">
      {/* Header */}
      <div className="p-4 border-b border-[#1E1F22] bg-[#2B2D31] flex flex-wrap gap-4 items-center justify-between">
        <div>
          <h3 className="font-semibold text-lg flex items-center gap-2">
            <span className="text-gray-400">#</span> ticket-{transcript.ticket_id}
          </h3>
          <p className="text-xs text-gray-400 mt-1">
            {transcript.panel_name} &bull; {transcript.category_name}
          </p>
        </div>
        <div className="flex gap-2">
          <Badge variant="neutral">Creator: {transcript.creator_username}</Badge>
          <Badge variant="danger">Closed by: {transcript.closed_by_username}</Badge>
        </div>
      </div>

      {/* Messages */}
      <div className="flex-1 overflow-y-auto p-4 space-y-6">
        {transcript.messages.map((msg, index) => {
          const isConsecutive =
            index > 0 &&
            transcript.messages[index - 1].author_id === msg.author_id &&
            new Date(msg.timestamp).getTime() - new Date(transcript.messages[index - 1].timestamp).getTime() < 300000;

          return (
            <div key={msg.id} className={`flex gap-4 ${isConsecutive ? "mt-1" : "mt-6"}`}>
              {!isConsecutive ? (
                <div className="flex-shrink-0 w-10 h-10 rounded-full bg-gray-600 overflow-hidden flex items-center justify-center">
                  {msg.author_avatar ? (
                    <img src={msg.author_avatar} alt="avatar" className="w-full h-full object-cover" />
                  ) : (
                    <span className="text-lg text-white font-bold">{msg.author_username[0].toUpperCase()}</span>
                  )}
                </div>
              ) : (
                <div className="w-10 flex-shrink-0 text-right pr-2 text-[10px] text-gray-500 opacity-0 hover:opacity-100">
                  {new Date(msg.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                </div>
              )}

              <div className="flex-1 min-w-0">
                {!isConsecutive && (
                  <div className="flex items-baseline gap-2 mb-1">
                    <span className={`font-semibold ${msg.is_staff ? "text-[#5865F2]" : "text-gray-100"}`}>
                      {msg.author_username}
                    </span>
                    {msg.is_staff && (
                      <span className="bg-[#5865F2] text-white text-[10px] uppercase font-bold px-1 rounded">
                        Staff
                      </span>
                    )}
                    <span className="text-xs text-gray-400">{formatTimestamp(msg.timestamp)}</span>
                  </div>
                )}
                
                <div className="text-gray-200 whitespace-pre-wrap break-words">{msg.content}</div>

                {msg.attachments && msg.attachments.length > 0 && (
                  <div className="mt-2 space-y-2">
                    {msg.attachments.map((att, i) => (
                      <div key={i} className="max-w-md bg-[#2B2D31] border border-[#1E1F22] rounded p-2 flex items-center gap-3">
                        <div className="flex-1 min-w-0">
                          <a href={att.url} target="_blank" rel="noreferrer" className="text-[#00A8FC] text-sm truncate block hover:underline">
                            {att.filename}
                          </a>
                          <p className="text-xs text-gray-400">{(att.size / 1024).toFixed(1)} KB</p>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          );
        })}

        {transcript.messages.length === 0 && (
          <div className="text-center text-gray-500 py-10">No messages in this transcript.</div>
        )}
        
        <div className="border-t border-gray-600 mt-6 pt-4 flex items-center gap-4">
           <div className="h-px bg-gray-600 flex-1"></div>
           <span className="text-gray-400 text-sm">Ticket Closed</span>
           <div className="h-px bg-gray-600 flex-1"></div>
        </div>
      </div>
    </div>
  );
}
