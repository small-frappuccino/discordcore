import { Route, Routes, Navigate } from "react-router-dom";
import { DashboardLayout } from "../pages/DashboardLayout";
import { CorePage } from "../pages/CorePage";
import { QOTDPage } from "../pages/QOTDPage";
import { ModerationPage } from "../pages/ModerationPage";
import { RolesPage } from "../pages/RolesPage";
import { PartnersPage } from "../pages/PartnersPage";
import { EmbedsPage } from "../pages/EmbedsPage";

export function AppRoutes() {
  return (
    <Routes>
        <Route path="/manage" element={<DashboardLayout />}>
          <Route index element={<Navigate to="/manage/core" replace />} />
          <Route path="core" element={<CorePage />} />
          <Route path="qotd" element={<QOTDPage />} />
          <Route path="moderation" element={<ModerationPage />} />
          <Route path="roles" element={<RolesPage />} />
          <Route path="partners" element={<PartnersPage />} />
          <Route path="embeds" element={<EmbedsPage />} />
        </Route>
        <Route path="*" element={<Navigate to="/manage" replace />} />
    </Routes>
  );
}
