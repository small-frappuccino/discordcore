import { memo, useState, useEffect } from "react";
import { Link, Outlet, useLocation, useParams } from "react-router-dom";
import { GuildProvider } from "../context/GuildContext";
import { ServerSelector } from "../components/layout/ServerSelector";
import { AccountSelector } from "../components/layout/AccountSelector";
import { useDashboardSession } from "../context/DashboardSessionContext";

const siteBrandIconSrc = "/favicon.ico";

import { CoreSettingsIcon } from "../components/icons/CoreSettingsIcon";
import { QOTDIcon } from "../components/icons/QOTDIcon";
import { ModerationIcon } from "../components/icons/ModerationIcon";
import { RolesIcon } from "../components/icons/RolesIcon";
import { PartnersIcon } from "../components/icons/PartnersIcon";
import { EmbedsIcon } from "../components/icons/EmbedsIcon";
import { TicketsIcon } from "../components/icons/TicketsIcon";
import { LoggingIcon } from "../components/icons/LoggingIcon";

import { useQueryClient } from "@tanstack/react-query";
import { guildFeatureQueryKey } from "../api/hooks/useGuildFeatures";
import { getGuildFeature } from "../api/domains/features";
import { partnerBoardQueryKey } from "../api/hooks/usePartners";
import { getPartnerBoard } from "../api/domains/partners";
import { getTicketsConfig } from "../api/domains/tickets";

type NavItem = {
  id: string;
  label: string;
  to: string;
  icon?: React.FC<React.SVGProps<SVGSVGElement>>;
};

const navigation: NavItem[] = [
  { id: "core", label: "Core Settings", to: "/core", icon: CoreSettingsIcon },
  { id: "qotd", label: "QOTD", to: "/qotd", icon: QOTDIcon },
  { id: "moderation", label: "Moderation", to: "/moderation", icon: ModerationIcon },
  { id: "roles", label: "Roles", to: "/roles", icon: RolesIcon },
  { id: "partners", label: "Partners", to: "/partners", icon: PartnersIcon },
  { id: "embeds", label: "Embeds", to: "/embeds", icon: EmbedsIcon },
  { id: "logging", label: "Logging", to: "/logging", icon: LoggingIcon },
  { id: "tickets", label: "Tickets", to: "/tickets", icon: TicketsIcon },
];

export const DashboardLayout = memo(function DashboardLayout() {
  const location = useLocation();
  const { guildId } = useParams<{ guildId: string }>();
  const { fetchDisplayBotProfile, displayBotProfile, client } = useDashboardSession();
  const queryClient = useQueryClient();
  const [brandIconError, setBrandIconError] = useState(false);

  useEffect(() => {
    if (guildId) {
      fetchDisplayBotProfile(guildId);
    }
  }, [guildId, fetchDisplayBotProfile]);

  const botName = displayBotProfile ? displayBotProfile.username : null;
  const botAvatar = displayBotProfile ? displayBotProfile.avatar_url : null;

  return (
    <div className="dashboard-layout">
      {/* Sidebar */}
      <aside className="shell-sidebar">
        <div className="shell-sidebar-header">
          <Link to="/manage" className="shell-brand">
            {botAvatar && !brandIconError ? (
              <img 
                src={botAvatar} 
                alt="Brand" 
                onError={() => setBrandIconError(true)}
              />
            ) : botName && !brandIconError ? (
              <div style={{ width: '28px', height: '28px', borderRadius: '50%', backgroundColor: 'var(--bg-surface-active)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '14px', fontWeight: 'bold' }}>{botName.charAt(0).toUpperCase()}</div>
            ) : brandIconError ? (
              <div style={{ width: '28px', height: '28px', borderRadius: '50%', backgroundColor: 'var(--bg-surface-active)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '14px', fontWeight: 'bold' }}>D</div>
            ) : (
              <img 
                src={siteBrandIconSrc} 
                alt="Brand" 
                onError={() => setBrandIconError(true)}
              />
            )}
            <span>{botName || "Discordcore"}</span>
          </Link>
        </div>

        <nav className="shell-nav">
          <div className="shell-nav-section-title">Features</div>
          {guildId && navigation.map((item) => {
            const fullPath = `/manage/${guildId}${item.to}`;
            const isActive = location.pathname.startsWith(fullPath);
            return (
              <Link
                key={item.id}
                to={fullPath}
                className={`shell-nav-link ${isActive ? "is-active" : ""}`}
                onMouseEnter={() => {
                  if (!client || !guildId) return;
                  if (item.id === "moderation") {
                    queryClient.prefetchQuery({ queryKey: guildFeatureQueryKey(client.getBaseUrl(), guildId, "automod"), queryFn: () => getGuildFeature(client, guildId, "automod") });
                    queryClient.prefetchQuery({ queryKey: guildFeatureQueryKey(client.getBaseUrl(), guildId, "logging"), queryFn: () => getGuildFeature(client, guildId, "logging") });
                  } else if (item.id === "partners") {
                    queryClient.prefetchQuery({ queryKey: partnerBoardQueryKey(client.getBaseUrl(), guildId), queryFn: () => getPartnerBoard(client, guildId) });
                  } else if (item.id === "tickets") {
                    queryClient.prefetchQuery({ queryKey: ["tickets-config", guildId], queryFn: () => getTicketsConfig(client, guildId) });
                  }
                }}
              >
                {item.icon && <item.icon className="shell-nav-icon" />}
                <span>{item.label}</span>
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
