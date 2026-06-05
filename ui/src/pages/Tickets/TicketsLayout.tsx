import { Outlet, NavLink, useLocation } from "react-router-dom";
import { useCurrentGuild } from "../../context/GuildContext";
import { buildTicketsTabs } from "../../app/routes";
import { PageHeader, Badge, PageContainer } from "../../components/ui";
import { Stack } from "../../components/layout";

export function TicketsLayout() {
  const { guildId: selectedGuildID } = useCurrentGuild();
  const location = useLocation();

  if (!selectedGuildID) {
    return <div>Select a server to manage tickets.</div>;
  }

  const tabs = buildTicketsTabs(selectedGuildID);

  return (
    <PageContainer>
      <Stack spacing="xl">
        <PageHeader
          title="Ticket System"
          description="Configure your fully customizable moderation ticket engine."
          badge={<Badge variant="success">Active</Badge>}
        />

        <div className="border-b border-surface-border">
          <nav className="-mb-px flex space-x-8" aria-label="Tabs">
            {tabs.map((tab) => {
              const isActive = location.pathname === tab.path || location.pathname.startsWith(`${tab.path}/`);
              return (
                <NavLink
                  key={tab.label}
                  to={tab.path}
                  className={`whitespace-nowrap pb-4 px-1 border-b-2 font-medium text-sm transition-colors ${
                    isActive
                      ? "border-primary text-primary"
                      : "border-transparent text-muted hover:text-foreground hover:border-surface-border"
                  }`}
                >
                  {tab.label}
                </NavLink>
              );
            })}
          </nav>
        </div>

        <div>
          <Outlet />
        </div>
      </Stack>
    </PageContainer>
  );
}
