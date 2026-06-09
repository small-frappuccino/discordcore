import { Navigate } from "react-router-dom";
import { PageContainer } from "../components/ui";
import { SettingsGroup, SettingsRow, ActionTrigger } from "../components/ui/tahoe";
import { useDashboardSession } from "../context/DashboardSessionContext";

export function LandingPage() {
  const { authState, beginLogin } = useDashboardSession();

  if (authState === "signed_in") {
    return <Navigate to="/manage" replace />;
  }

  return (
    <PageContainer>
      <div className="flex flex-col min-h-screen w-full pb-20">
        {/* Navigation Header */}
        <div className="flex items-center justify-between w-full py-6 mb-8">
          <div className="flex items-center gap-3">
            <div className="w-7 h-7 rounded-full bg-bg-surface-active flex items-center justify-center text-sm font-bold text-text-primary">
              D
            </div>
            <span className="font-semibold text-text-primary">discordcore</span>
          </div>
          <div>
            <ActionTrigger onClick={() => void beginLogin()}>
              Login with Discord
            </ActionTrigger>
          </div>
        </div>

        {/* Primary Viewport */}
        <div className="flex flex-1 flex-col items-center justify-center w-full">
          <div className="w-full max-w-2xl">
            <SettingsGroup>
              <SettingsRow 
                title="Discordcore Configuration Panel" 
                description="Sign in with your authorized Discord account to manage your bot instances, configuration overrides, and operational feature routing logic."
                isMultiline
                control={
                  <ActionTrigger onClick={() => void beginLogin()}>
                    Authenticate
                  </ActionTrigger>
                }
              />
            </SettingsGroup>
          </div>
        </div>
      </div>
    </PageContainer>
  );
}
