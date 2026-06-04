import { BrowserRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AppRoutes } from "./app/AppRoutes";
import { DashboardSessionProvider } from "./context/DashboardSessionContext";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      refetchOnWindowFocus: true,
      retry: 1,
      staleTime: 5 * 60 * 1000, // 5 minutes by default
    },
  },
});

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <DashboardSessionProvider>
          <AppRoutes />
        </DashboardSessionProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}
