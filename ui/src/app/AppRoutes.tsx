import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { DashboardLayout } from "../pages/DashboardLayout";
import { CommandsPage } from "../pages/CommandsPage";
import { HomePage } from "../pages/HomePage";
import { LoggingCategoryPage } from "../pages/LoggingCategoryPage";
import { ModerationPage } from "../pages/ModerationPage";
import { RolesPage } from "../pages/RolesPage";
import { StatsPage } from "../pages/StatsPage";
import { ControlPanelPage } from "../pages/ControlPanelPage";
import { PartnerBoardProvider } from "../features/partner-board/PartnerBoardContext";
import { PartnerBoardLayout } from "../features/partner-board/PartnerBoardLayout";
import { PartnerBoardEntriesPage } from "../features/partner-board/PartnerBoardEntriesPage";
import { PartnerBoardLayoutPage } from "../features/partner-board/PartnerBoardLayoutPage";
import { PartnerBoardDeliveryPage } from "../features/partner-board/PartnerBoardDeliveryPage";
import { QOTDProvider } from "../features/qotd/QOTDContext";
import { QOTDLayout } from "../features/qotd/QOTDLayout";
import { QOTDSettingsPage } from "../features/qotd/QOTDSettingsPage";
import { QOTDQuestionsPage } from "../features/qotd/QOTDQuestionsPage";
import { QOTDCollectorPage } from "../features/qotd/QOTDCollectorPage";
import { appRoutes, mapLegacyDashboardPathForGuild } from "./routes";
import { ManageIndexPage } from "../pages/ManageIndexPage";
import { LandingPage } from "../pages/LandingPage";
import { useDashboardSession } from "../context/DashboardSessionContext";

export function AppRoutes() {
  return (
    <Routes>
      <Route path={appRoutes.manage} element={<DashboardLayout />}>
        <Route index element={<ManageIndexPage />} />
        <Route path=":guildId">
          <Route index element={<Navigate replace to="home" />} />
          <Route path="home" element={<HomePage />} />
          <Route
            path="control-panel"
            element={<GuildRedirect to="core/control-panel" />}
          />
          <Route
            path="commands"
            element={<GuildRedirect to="core/commands" />}
          />
          <Route
            path="logging"
            element={<GuildRedirect to="moderation/logging" />}
          />
          <Route path="stats" element={<GuildRedirect to="core/stats" />} />
          <Route
            path="roles-members"
            element={<GuildRedirect to="roles/autorole" />}
          />

          <Route path="core">
            <Route index element={<Navigate replace to="control-panel" />} />
            <Route path="control-panel" element={<ControlPanelPage />} />
            <Route path="stats" element={<StatsPage />} />
            <Route path="commands" element={<CommandsPage />} />
          </Route>

          <Route path="moderation">
            <Route index element={<Navigate replace to="moderation" />} />
            <Route path="moderation" element={<ModerationPage />} />
            <Route path="logging" element={<LoggingCategoryPage />} />
          </Route>

          <Route
            path="partner-board"
            element={
              <PartnerBoardProvider>
                <PartnerBoardLayout />
              </PartnerBoardProvider>
            }
          >
            <Route index element={<Navigate replace to="entries" />} />
            <Route path="entries" element={<PartnerBoardEntriesPage />} />
            <Route path="layout" element={<PartnerBoardLayoutPage />} />
            <Route path="delivery" element={<PartnerBoardDeliveryPage />} />
            <Route
              path="activity"
              element={<Navigate replace to="../entries" />}
            />
          </Route>

          <Route
            path="qotd"
            element={
              <QOTDProvider>
                <QOTDLayout />
              </QOTDProvider>
            }
          >
            <Route index element={<Navigate replace to="settings" />} />
            <Route path="settings" element={<QOTDSettingsPage />} />
            <Route path="questions" element={<QOTDQuestionsPage />} />
            <Route path="collector" element={<QOTDCollectorPage />} />
          </Route>

          <Route path="roles">
            <Route index element={<Navigate replace to="autorole" />} />
            <Route path="autorole" element={<RolesPage />} />
            <Route path="level-roles" element={<RolesPage />} />
          </Route>
        </Route>
      </Route>

      <Route path="/dashboard" element={<LegacyDashboardRedirect />} />
      <Route path="/dashboard/*" element={<LegacyDashboardRedirect />} />
      <Route path={appRoutes.landing} element={<LandingPage />} />
      <Route path="*" element={<Navigate replace to={appRoutes.manage} />} />
    </Routes>
  );
}

function GuildRedirect({ to }: { to: string }) {
  return <Navigate replace to={to} />;
}

function LegacyDashboardRedirect() {
  const location = useLocation();
  const { authState, manageableGuilds, selectedGuildID } =
    useDashboardSession();
  const fallbackGuildID =
    selectedGuildID.trim() !== ""
      ? selectedGuildID.trim()
      : (manageableGuilds[0]?.id?.trim() ?? "");

  if (authState === "checking") {
    return null;
  }

  return (
    <Navigate
      replace
      to={`${mapLegacyDashboardPathForGuild(location.pathname, fallbackGuildID)}${location.search}${location.hash}`}
    />
  );
}
