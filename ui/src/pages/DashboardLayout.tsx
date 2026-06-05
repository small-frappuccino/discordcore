import { memo } from "react";
import { Link, Outlet, useLocation, useParams } from "react-router-dom";
import { GuildProvider } from "../context/GuildContext";
import { ServerSelector } from "../components/layout/ServerSelector";
import { AccountSelector } from "../components/layout/AccountSelector";

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
  { id: "tickets", label: "Tickets", to: "/tickets" },
];

export const DashboardLayout = memo(function DashboardLayout() {
  const location = useLocation();
  const { guildId } = useParams<{ guildId: string }>();

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
            <ServerSelector />

            {/* Account Selector */}
            <AccountSelector />
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
});
