import { useState, useRef, useEffect, memo, useMemo } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useDashboardSession } from "../../context/DashboardSessionContext";

export const ServerSelector = memo(function ServerSelector() {
  const navigate = useNavigate();
  const { guildId } = useParams<{ guildId: string }>();
  const { accessibleGuilds, manageableGuilds } = useDashboardSession();

  const [isServerMenuOpen, setIsServerMenuOpen] = useState(false);
  const serverMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (serverMenuRef.current && !serverMenuRef.current.contains(event.target as Node)) {
        setIsServerMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const currentGuild = accessibleGuilds?.find((g) => g.id === guildId) || manageableGuilds?.find((g) => g.id === guildId);
  const serverTitle = currentGuild ? currentGuild.name : (guildId ? `Server ${guildId}` : "Select server");
  const serverSubtitle = "Choose workspace";

  // Combine and deduplicate guilds for the server selector using useMemo
  const uniqueGuilds = useMemo(() => {
    const allGuildsMap = new Map();
    accessibleGuilds?.forEach(g => allGuildsMap.set(g.id, g));
    manageableGuilds?.forEach(g => allGuildsMap.set(g.id, g));
    return Array.from(allGuildsMap.values());
  }, [accessibleGuilds, manageableGuilds]);

  return (
    <div className="relative" ref={serverMenuRef}>
      <button 
        className="shell-trigger-btn"
        onClick={() => setIsServerMenuOpen(!isServerMenuOpen)}
      >
        <div className="shell-trigger-info">
          <span className="shell-trigger-title">{serverTitle}</span>
          <span className="shell-trigger-subtitle">{serverSubtitle}</span>
        </div>
        <span className="shell-trigger-chevron">v</span>
      </button>

      <div 
        className={`shell-dropdown transition-all duration-200 ease-out origin-top-right ${
          isServerMenuOpen ? 'opacity-100 scale-100 pointer-events-auto' : 'opacity-0 scale-95 pointer-events-none'
        }`}
      >
        {uniqueGuilds.length === 0 ? (
          <div className="p-2 text-sm text-muted">No servers found</div>
        ) : (
          uniqueGuilds.map((g) => (
            <button
              key={g.id}
              className="shell-dropdown-item"
              onClick={() => {
                setIsServerMenuOpen(false);
                navigate(`/manage/${g.id}/core`);
              }}
            >
              {g.icon ? (
                <img src={`https://cdn.discordapp.com/icons/${g.id}/${g.icon}.png`} alt="" className="w-5 h-5 rounded-full" />
              ) : (
                <div className="w-5 h-5 rounded-full bg-surface-active flex items-center justify-center text-xs">
                  {g.name.charAt(0)}
                </div>
              )}
              {g.name}
            </button>
          ))
        )}
      </div>
    </div>
  );
});
