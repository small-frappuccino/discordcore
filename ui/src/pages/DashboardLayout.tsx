import { useState, useRef, useEffect } from "react";
import { Link, Outlet, useLocation, useParams, useNavigate } from "react-router-dom";
import { GuildProvider } from "../context/GuildContext";
import { useDashboardSession } from "../context/DashboardSessionContext";

const siteBrandIconSrc = "/favicon.ico";

type NavItem = {
  id: string;
  label: string;
  to: string;
};

const navigation: NavItem[] = [
  { id: "core", label: "Core Settings", to: "/core" },
  { id: "qotd", label: "QOTD", to: "/qotd" },
  { id: "moderation", label: "Moderation", to: "/moderation" },
  { id: "roles", label: "Roles", to: "/roles" },
  { id: "partners", label: "Partners", to: "/partners" },
  { id: "embeds", label: "Embeds", to: "/embeds" },
];

export function DashboardLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const { guildId } = useParams<{ guildId: string }>();
  const { 
    session, 
    sessionAvatarURL, 
    accessibleGuilds, 
    manageableGuilds,
    logout 
  } = useDashboardSession();

  const [isServerMenuOpen, setIsServerMenuOpen] = useState(false);
  const [isAccountMenuOpen, setIsAccountMenuOpen] = useState(false);

  const serverMenuRef = useRef<HTMLDivElement>(null);
  const accountMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (serverMenuRef.current && !serverMenuRef.current.contains(event.target as Node)) {
        setIsServerMenuOpen(false);
      }
      if (accountMenuRef.current && !accountMenuRef.current.contains(event.target as Node)) {
        setIsAccountMenuOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const currentGuild = accessibleGuilds?.find((g) => g.id === guildId) || manageableGuilds?.find((g) => g.id === guildId);

  const accountTitle = session?.user?.username || "Unknown User";
  const avatarUrl = sessionAvatarURL || "https://cdn.discordapp.com/embed/avatars/0.png";
  
  const serverTitle = currentGuild ? currentGuild.name : (guildId ? `Server ${guildId}` : "Select server");
  const serverSubtitle = "Choose workspace";

  // Combine and deduplicate guilds for the server selector
  const allGuildsMap = new Map();
  accessibleGuilds?.forEach(g => allGuildsMap.set(g.id, g));
  manageableGuilds?.forEach(g => allGuildsMap.set(g.id, g));
  const uniqueGuilds = Array.from(allGuildsMap.values());

  return (
    <div className="dashboard-layout">
      {/* Sidebar */}
      <aside className="shell-sidebar">
        <div className="shell-sidebar-header">
          <Link to="/manage" className="shell-brand">
            <img src={siteBrandIconSrc} alt="Brand" />
            <span>Discordcore</span>
          </Link>
        </div>

        <nav className="shell-nav">
          <div className="shell-nav-section-title">Features</div>
          {navigation.map((item) => {
            const fullPath = `/manage/${guildId}${item.to}`;
            const isActive = location.pathname.startsWith(fullPath);
            return (
              <Link
                key={item.id}
                to={fullPath}
                className={`shell-nav-link ${isActive ? "is-active" : ""}`}
              >
                {item.label}
              </Link>
            );
          })}
        </nav>
      </aside>

      {/* Main Content Area */}
      <main className="shell-main">
        {/* Topbar */}
        <header className="shell-topbar">
          <div className="shell-topbar-left">
            {/* Context/Breadcrumb area if needed */}
          </div>
          <div className="shell-topbar-right">
            {/* Server Selector */}
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

              {isServerMenuOpen && (
                <div className="shell-dropdown">
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
              )}
            </div>

            {/* Account Selector */}
            <div className="relative" ref={accountMenuRef}>
              <button 
                className="shell-trigger-btn"
                onClick={() => setIsAccountMenuOpen(!isAccountMenuOpen)}
              >
                <div className="shell-trigger-avatar">
                  <img src={avatarUrl} alt="Avatar" />
                </div>
                <div className="shell-trigger-info">
                  <span className="shell-trigger-title">{accountTitle}</span>
                </div>
                <span className="shell-trigger-chevron">v</span>
              </button>

              {isAccountMenuOpen && (
                <div className="shell-dropdown">
                  <div className="px-3 py-2 border-b border-subtle mb-1">
                    <div className="text-sm font-semibold">{accountTitle}</div>
                    <div className="text-xs text-muted">{session?.user?.id}</div>
                  </div>
                  <button
                    className="shell-dropdown-item danger"
                    onClick={() => {
                      setIsAccountMenuOpen(false);
                      logout();
                    }}
                  >
                    Log Out
                  </button>
                </div>
              )}
            </div>
          </div>
        </header>

        {/* Page Content */}
        <div className="shell-content">
          <GuildProvider>
            <Outlet />
          </GuildProvider>
        </div>
      </main>
    </div>
  );
}
