import { Link, Outlet, useLocation, useParams } from "react-router-dom";
import { GuildProvider } from "../context/GuildContext";

// Placeholder Contexts/Imports
// We will build real contexts for Auth/Guild in the next steps.
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
  const { guildId } = useParams<{ guildId: string }>();

  // Temporary mock states
  const accountTitle = "alice";
  const serverTitle = guildId ? `Server ${guildId}` : "Select server";
  const serverSubtitle = "Choose workspace";

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
            <button className="shell-trigger-btn">
              <div className="shell-trigger-info">
                <span className="shell-trigger-title">{serverTitle}</span>
                <span className="shell-trigger-subtitle">{serverSubtitle}</span>
              </div>
              <span className="shell-trigger-chevron">v</span>
            </button>

            {/* Account Selector */}
            <button className="shell-trigger-btn">
              <div className="shell-trigger-avatar">
                <img src="https://cdn.discordapp.com/embed/avatars/0.png" alt="Avatar" />
              </div>
              <div className="shell-trigger-info">
                <span className="shell-trigger-title">{accountTitle}</span>
              </div>
              <span className="shell-trigger-chevron">v</span>
            </button>
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
