import { Navigate, Route, Routes } from "react-router-dom";
import { appRoutes } from "./routes";
import { DashboardLayout } from "../pages/DashboardLayout";
import { CommandsPage } from "../pages/CommandsPage";
import { FeatureCategoryPage } from "../pages/FeatureCategoryPage";
import { LandingPage } from "../pages/LandingPage";
import { LoggingCategoryPage } from "../pages/LoggingCategoryPage";
import { ModerationPage } from "../pages/ModerationPage";
import { HomePage } from "../pages/HomePage";
import { RolesPage } from "../pages/RolesPage";
import { SettingsPage } from "../pages/SettingsPage";
import { StatsPage } from "../pages/StatsPage";
import { PartnerBoardProvider } from "../features/partner-board/PartnerBoardContext";
import { PartnerBoardLayout } from "../features/partner-board/PartnerBoardLayout";
import { PartnerBoardEntriesPage } from "../features/partner-board/PartnerBoardEntriesPage";
import { PartnerBoardLayoutPage } from "../features/partner-board/PartnerBoardLayoutPage";
import { PartnerBoardDeliveryPage } from "../features/partner-board/PartnerBoardDeliveryPage";

export function AppRoutes() {
  return (
    <Routes>
      <Route path={appRoutes.landing} element={<LandingPage />} />
      <Route path={appRoutes.dashboard} element={<DashboardLayout />}>
        <Route index element={<Navigate replace to="home" />} />
        <Route path="home" element={<HomePage />} />
        <Route
          path="overview"
          element={<Navigate replace to={appRoutes.dashboardHome} />}
        />
        <Route
          path="control-panel"
          element={<Navigate replace to={appRoutes.dashboardCoreControlPanel} />}
        />
        <Route
          path="commands"
          element={<Navigate replace to={appRoutes.dashboardCoreCommands} />}
        />
        <Route
          path="logging"
          element={<Navigate replace to={appRoutes.dashboardModerationLogging} />}
        />
        <Route
          path="stats"
          element={<Navigate replace to={appRoutes.dashboardCoreStats} />}
        />
        <Route
          path="maintenance"
          element={<Navigate replace to={appRoutes.settingsAdvanced} />}
        />
        <Route
          path="roles-members"
          element={<Navigate replace to={appRoutes.dashboardRolesAutorole} />}
        />
        <Route
          path="automations"
          element={<Navigate replace to={appRoutes.dashboardHomePlanned} />}
        />
        <Route
          path="activity"
          element={<Navigate replace to={appRoutes.dashboardHomePlanned} />}
        />
        <Route path="settings" element={<SettingsPage />} />
        <Route path="feature-areas/:areaId" element={<FeatureCategoryPage />} />

        <Route path="core">
          <Route index element={<Navigate replace to="control-panel" />} />
          <Route
            path="control-panel"
            element={<Navigate replace to={appRoutes.settingsPermissions} />}
          />
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
            element={<Navigate replace to={appRoutes.partnerBoardEntries} />}
          />
        </Route>

        <Route path="roles">
          <Route index element={<Navigate replace to="autorole" />} />
          <Route path="autorole" element={<RolesPage />} />
          <Route path="level-roles" element={<RolesPage />} />
        </Route>
      </Route>
      <Route
        path="*"
        element={<Navigate replace to={appRoutes.dashboardHome} />}
      />
    </Routes>
  );
}
