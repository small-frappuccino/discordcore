import { Navigate, Route, Routes } from "react-router-dom";
import { appRoutes } from "./routes";
import { DashboardLayout } from "../pages/DashboardLayout";
import { LandingPage } from "../pages/LandingPage";
import { OverviewPage } from "../pages/OverviewPage";
import { SettingsPage } from "../pages/SettingsPage";
import { PartnerBoardProvider } from "../features/partner-board/PartnerBoardContext";
import { PartnerBoardLayout } from "../features/partner-board/PartnerBoardLayout";
import { PartnerBoardEntriesPage } from "../features/partner-board/PartnerBoardEntriesPage";
import { PartnerBoardLayoutPage } from "../features/partner-board/PartnerBoardLayoutPage";
import { PartnerBoardDeliveryPage } from "../features/partner-board/PartnerBoardDeliveryPage";

export function AppRoutes() {
  const roadmapRedirect = `${appRoutes.dashboardOverview}#roadmap`;

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
          <Route
            path="activity"
            element={<Navigate replace to={appRoutes.partnerBoardEntries} />}
          />
        </Route>
        <Route
          path="moderation"
          element={<Navigate replace to={roadmapRedirect} />}
        />
        <Route
          path="automations"
          element={<Navigate replace to={roadmapRedirect} />}
        />
        <Route
          path="activity"
          element={<Navigate replace to={roadmapRedirect} />}
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
