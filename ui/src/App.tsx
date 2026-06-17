import { BrowserRouter } from "react-router-dom";
import { QueryClient } from "@tanstack/react-query";
import { PersistQueryClientProvider } from '@tanstack/react-query-persist-client';
import { createSyncStoragePersister } from '@tanstack/query-sync-storage-persister';
import { Toaster } from "react-hot-toast";
import { ErrorBoundary } from "react-error-boundary";
import { AppRoutes } from "./app/AppRoutes";
import { DashboardSessionProvider } from "./context/DashboardSessionContext";
import { UserPreferencesProvider } from "./context/UserPreferencesContext";
import { SettingsModalProvider } from "./context/SettingsModalContext";
import { ErrorFallback } from "./components/ui/ErrorFallback/ErrorFallback";
import { logger } from "./lib/logger";
import { initPerformanceTelemetry } from "./lib/telemetry";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: true,
      retry: 1,
      staleTime: 5 * 60 * 1000, // 5 minutes by default
      gcTime: 24 * 60 * 60 * 1000, // 24 hours
    },
  },
});

const persister = createSyncStoragePersister({
  storage: typeof window !== "undefined" ? window.localStorage : undefined,
  key: "discordcore_query_cache",
});

initPerformanceTelemetry();

if (typeof window !== "undefined") {
  window.addEventListener("unhandledrejection", (event) => {
    logger.error("Unhandled Promise Rejection", {
      reason: event.reason instanceof Error ? event.reason.message : event.reason,
      stack: event.reason instanceof Error ? event.reason.stack : undefined,
    });
  });
}

export default function App() {
  return (
    <ErrorBoundary 
      FallbackComponent={ErrorFallback}
      onError={(error, info) => {
        logger.error("React ErrorBoundary caught an error", {
          error: error instanceof Error ? error.message : String(error),
          stack: error instanceof Error ? error.stack : undefined,
          componentStack: info.componentStack,
        });
      }}
    >
      <PersistQueryClientProvider client={queryClient} persistOptions={{ persister }}>
        <BrowserRouter>
          <DashboardSessionProvider>
            <UserPreferencesProvider>
              <SettingsModalProvider>
                <AppRoutes />
                <Toaster position="bottom-right" />
              </SettingsModalProvider>
            </UserPreferencesProvider>
          </DashboardSessionProvider>
        </BrowserRouter>
      </PersistQueryClientProvider>
    </ErrorBoundary>
  );
}
