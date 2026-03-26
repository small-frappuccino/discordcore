import { Navigate, Route, Routes } from "react-router-dom";
import { appRoutes } from "./routes";
import { DashboardLayout } from "../pages/DashboardLayout";
import { CommandsPage } from "../pages/CommandsPage";
import { HomePage } from "../pages/HomePage";
import { LandingPage } from "../pages/LandingPage";
import { LoggingCategoryPage } from "../pages/LoggingCategoryPage";
import { ModerationPage } from "../pages/ModerationPage";
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
      <Route path="/dashboard" element={<DashboardLayout />}>
        <Route index element={<Navigate replace to="home" />} />
        <Route
          path="control-panel"
          element={<Navigate replace to={appRoutes.dashboardHome} />}
        />
        <Route path="home" element={<HomePage />} />
        <Route
          path="overview"
          element={<Navigate replace to={appRoutes.dashboardHome} />}
        />
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
        <Route path="commands" element={<CommandsPage />} />
        <Route path="moderation" element={<ModerationPage />} />
        <Route path="logging" element={<LoggingCategoryPage />} />
        <Route path="roles" element={<RolesPage />} />
        <Route
          path="roles-members"
          element={<Navigate replace to={appRoutes.roles} />}
        />
        <Route
          path="maintenance"
          element={<Navigate replace to={appRoutes.settingsAdvanced} />}
        />
        <Route path="stats" element={<StatsPage />} />
        <Route
          path="automations"
          element={<Navigate replace to={appRoutes.dashboardHomePlanned} />}
        />
        <Route
          path="activity"
          element={<Navigate replace to={appRoutes.dashboardHomePlanned} />}
        />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
      <Route
        path="*"
        element={<Navigate replace to={appRoutes.dashboardHome} />}
      />
    </Routes>
  );
}
