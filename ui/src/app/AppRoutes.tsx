import { lazy, Suspense } from "react";
import { Route, Routes, Navigate } from "react-router-dom";

const DashboardLayout = lazy(() => import("../pages/DashboardLayout").then(m => ({ default: m.DashboardLayout })));
const CorePage = lazy(() => import("../pages/CorePage").then(m => ({ default: m.CorePage })));
const QOTDPage = lazy(() => import("../pages/QOTDPage").then(m => ({ default: m.QOTDPage })));
const ModerationPage = lazy(() => import("../pages/ModerationPage").then(m => ({ default: m.ModerationPage })));
const RolesPage = lazy(() => import("../pages/RolesPage").then(m => ({ default: m.RolesPage })));
const PartnersPage = lazy(() => import("../pages/PartnersPage").then(m => ({ default: m.PartnersPage })));
const EmbedsPage = lazy(() => import("../pages/EmbedsPage").then(m => ({ default: m.EmbedsPage })));
const TahoeMockPage = lazy(() => import("../pages/TahoeMockPage").then(m => ({ default: m.TahoeMockPage })));
const TicketsLayout = lazy(() => import("../pages/Tickets/TicketsLayout").then(m => ({ default: m.TicketsLayout })));
const TicketsPanelsPage = lazy(() => import("../pages/Tickets/TicketsPanelsPage").then(m => ({ default: m.TicketsPanelsPage })));
const TicketsFormsPage = lazy(() => import("../pages/Tickets/TicketsFormsPage").then(m => ({ default: m.TicketsFormsPage })));
const TicketsTranscriptsPage = lazy(() => import("../pages/Tickets/TicketsTranscriptsPage").then(m => ({ default: m.TicketsTranscriptsPage })));
const TicketsSettingsPage = lazy(() => import("../pages/Tickets/TicketsSettingsPage").then(m => ({ default: m.TicketsSettingsPage })));
const LandingPage = lazy(() => import("../pages/LandingPage").then(m => ({ default: m.LandingPage })));

export function AppRoutes() {
  return (
    <Suspense fallback={<div className="flex h-screen items-center justify-center p-4"><span className="text-sm text-surface-400">Carregando...</span></div>}>
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
    </Suspense>
  );
}
