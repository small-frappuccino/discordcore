import { Navigate, Route, Routes } from "react-router-dom";
import { appRoutes } from "./routes";
import { DashboardLayout } from "../pages/DashboardLayout";
import { LandingPage } from "../pages/LandingPage";
import { OverviewPage } from "../pages/OverviewPage";
import { PlaceholderPage } from "../pages/PlaceholderPage";
import { SettingsPage } from "../pages/SettingsPage";
import { PartnerBoardProvider } from "../features/partner-board/PartnerBoardContext";
import { PartnerBoardLayout } from "../features/partner-board/PartnerBoardLayout";
import { PartnerBoardEntriesPage } from "../features/partner-board/PartnerBoardEntriesPage";
import { PartnerBoardLayoutPage } from "../features/partner-board/PartnerBoardLayoutPage";
import { PartnerBoardDeliveryPage } from "../features/partner-board/PartnerBoardDeliveryPage";
import { PartnerBoardActivityPage } from "../features/partner-board/PartnerBoardActivityPage";

export function AppRoutes() {
  return (
    <Routes>
      <Route path={appRoutes.landing} element={<LandingPage />} />
      <Route path="/dashboard" element={<DashboardLayout />}>
        <Route index element={<Navigate replace to="overview" />} />
        <Route
          path="control-panel"
          element={<Navigate replace to={appRoutes.partnerBoardEntries} />}
        />
        <Route path="overview" element={<OverviewPage />} />
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
          <Route path="activity" element={<PartnerBoardActivityPage />} />
        </Route>
        <Route
          path="moderation"
          element={
            <PlaceholderPage
              title="Moderation"
              description="Moderation rules, queues, and reports will live here once the dashboard expands beyond Partner Board."
            />
          }
        />
        <Route
          path="automations"
          element={
            <PlaceholderPage
              title="Automations"
              description="Scheduled workflows and automation runs are deferred until the dashboard has the app shell in place."
            />
          }
        />
        <Route
          path="activity"
          element={
            <PlaceholderPage
              title="Activity Log"
              description="Cross-feature audit history is planned for a later phase after feature-specific activity contracts exist."
            />
          }
        />
        <Route path="settings" element={<SettingsPage />} />
      </Route>
      <Route
        path="*"
        element={<Navigate replace to={appRoutes.dashboardOverview} />}
      />
    </Routes>
  );
}
