import { Navigate } from "react-router-dom";
import { PageContainer, PageHeader } from "../components/ui";
import { SettingsGroup, SettingsRow, ActionTrigger } from "../components/ui/tahoe";
import { Stack } from "../components/layout";
import { useDashboardSession } from "../context/DashboardSessionContext";

export function LandingPage() {
  const { authState, beginLogin } = useDashboardSession();

  if (authState === "signed_in") {
    return <Navigate to="/manage" replace />;
  }

  return (
    <PageContainer>
      <Stack spacing="xl">
        <PageHeader>
          <PageHeader.TitleRow>
            <PageHeader.Title>Discordcore Dashboard</PageHeader.Title>
          </PageHeader.TitleRow>
          <PageHeader.Description>Manage your bot instances, configuration, and operational feature routing.</PageHeader.Description>
        </PageHeader>
        
        <div className="max-w-xl">
          <SettingsGroup>
            <SettingsRow 
              title="Authentication" 
              description="Sign in with your Discord account to access the control panel."
              control={
                <ActionTrigger onClick={() => void beginLogin()}>
                  Sign In with Discord
                </ActionTrigger>
              }
            />
          </SettingsGroup>
        </div>
      </Stack>
    </PageContainer>
  );
}
