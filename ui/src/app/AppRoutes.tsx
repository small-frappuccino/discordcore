import { Route, Routes, Navigate } from "react-router-dom";
import { DashboardLayout } from "../pages/DashboardLayout";
import { CorePage } from "../pages/CorePage";
import { QOTDPage } from "../pages/QOTDPage";
import { ModerationPage } from "../pages/ModerationPage";
import { RolesPage } from "../pages/RolesPage";
import { PartnersPage } from "../pages/PartnersPage";
import { EmbedsPage } from "../pages/EmbedsPage";
import { TahoeMockPage } from "../pages/TahoeMockPage";
import { TicketsLayout } from "../pages/Tickets/TicketsLayout";
import { TicketsPanelsPage } from "../pages/Tickets/TicketsPanelsPage";
import { TicketsFormsPage } from "../pages/Tickets/TicketsFormsPage";
import { TicketsTranscriptsPage } from "../pages/Tickets/TicketsTranscriptsPage";
import { TicketsSettingsPage } from "../pages/Tickets/TicketsSettingsPage";
import { LandingPage } from "../pages/LandingPage";

export function AppRoutes() {
  return (
    <Routes>
        <Route path="/" element={<LandingPage />} />
        <Route path="/manage/tahoe" element={<TahoeMockPage />} />
        <Route path="/manage" element={<DashboardLayout />}>
          <Route path=":guildId">
            <Route index element={<Navigate to="core" replace />} />
            <Route path="core" element={<CorePage />} />
            <Route path="qotd" element={<QOTDPage />} />
            <Route path="moderation" element={<ModerationPage />} />
            <Route path="roles" element={<RolesPage />} />
            <Route path="partners" element={<PartnersPage />} />
            <Route path="embeds" element={<EmbedsPage />} />
            <Route path="tickets" element={<TicketsLayout />}>
              <Route index element={<Navigate to="panels" replace />} />
              <Route path="panels" element={<TicketsPanelsPage />} />
              <Route path="forms" element={<TicketsFormsPage />} />
              <Route path="transcripts" element={<TicketsTranscriptsPage />} />
              <Route path="settings" element={<TicketsSettingsPage />} />
            </Route>
          </Route>
        </Route>
    </Routes>
  );
}
