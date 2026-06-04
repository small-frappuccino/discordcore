import { Route, Routes, Navigate } from "react-router-dom";
import { DashboardLayout } from "../pages/DashboardLayout";
import { CorePage } from "../pages/CorePage";
import { QOTDPage } from "../pages/QOTDPage";
import { ModerationPage } from "../pages/ModerationPage";
import { RolesPage } from "../pages/RolesPage";
import { PartnersPage } from "../pages/PartnersPage";
import { EmbedsPage } from "../pages/EmbedsPage";
import { useDashboardSession } from "../context/DashboardSessionContext";

function ManageIndexRedirect() {
  const { accessibleGuilds, manageableGuilds, authState } = useDashboardSession();
  
  if (authState === "checking") return <div>Loading...</div>;
  if (authState !== "signed_in") return <Navigate to="/" replace />;
  
  const firstGuild = accessibleGuilds?.[0] || manageableGuilds?.[0];
  if (firstGuild) {
    return <Navigate to={`/manage/${firstGuild.id}/core`} replace />;
  }
  return <div className="p-4 text-center">No available servers found. Please invite the bot first.</div>;
}

export function AppRoutes() {
  return (
    <Routes>
        <Route path="/manage" element={<ManageIndexRedirect />} />
        <Route path="/manage/:guildId" element={<DashboardLayout />}>
          <Route index element={<Navigate to="core" replace />} />
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
