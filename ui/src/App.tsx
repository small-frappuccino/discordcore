import { BrowserRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Toaster } from "react-hot-toast";
import { ErrorBoundary } from "react-error-boundary";
import { AppRoutes } from "./app/AppRoutes";
import { DashboardSessionProvider } from "./context/DashboardSessionContext";
import { ErrorFallback } from "./components/ui/ErrorFallback/ErrorFallback";
import { logger } from "./lib/logger";
import { initPerformanceTelemetry } from "./lib/telemetry";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: true,
      retry: 1,
      staleTime: 5 * 60 * 1000, // 5 minutes by default
    },
  },
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
      <QueryClientProvider client={queryClient}>
        <BrowserRouter>
          <DashboardSessionProvider>
            <AppRoutes />
            <Toaster position="bottom-right" />
          </DashboardSessionProvider>
        </BrowserRouter>
      </QueryClientProvider>
    </ErrorBoundary>
  );
}
