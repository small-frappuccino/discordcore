import { Outlet, NavLink, useLocation } from "react-router-dom";
import { useCurrentGuild } from "../../context/GuildContext";
import { buildTicketsTabs } from "../../app/routes";
import { PageHeader, Badge, PageContainer } from "../../components/ui";

export function TicketsLayout() {
  const { guildId: selectedGuildID } = useCurrentGuild();
  const location = useLocation();

  if (!selectedGuildID) {
    return <div>Select a server to manage tickets.</div>;
  }

  const tabs = buildTicketsTabs(selectedGuildID);

  return (
    <PageContainer>
      <PageHeader>
        <PageHeader.TitleRow>
          <PageHeader.Title>Ticket System</PageHeader.Title>
          <Badge variant="success">Active</Badge>
        </PageHeader.TitleRow>
        <PageHeader.Description>Configure your fully customizable moderation ticket engine.</PageHeader.Description>
      </PageHeader>

      <div className="mt-8 border-b border-surface-border">
        <nav className="-mb-px flex space-x-8" aria-label="Tabs">
          {tabs.map((tab) => {
            const isActive = location.pathname === tab.path || location.pathname.startsWith(`${tab.path}/`);
            return (
              <NavLink
                key={tab.label}
                to={tab.path}
                className={`whitespace-nowrap pb-4 pt-2 px-3 border-b-2 font-medium text-sm transition-all duration-200 ease-out active:scale-[0.98] rounded-t-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-1 focus-visible:ring-offset-base ${
                  isActive
                    ? "border-primary text-primary bg-white/5"
                    : "border-transparent text-muted hover:text-foreground hover:border-surface-border hover:bg-white/5"
                }`}
              >
                {tab.label}
              </NavLink>
            );
          })}
        </nav>
      </div>

      <div className="mt-8">
        <Outlet />
      </div>
    </PageContainer>
  );
}
