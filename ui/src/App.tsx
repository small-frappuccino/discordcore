import { BrowserRouter } from "react-router-dom";
import { AppRoutes } from "./app/AppRoutes";
import { DashboardSessionProvider } from "./context/DashboardSessionContext";

export default function App() {
  return (
    <BrowserRouter>
      <DashboardSessionProvider>
        <AppRoutes />
      </DashboardSessionProvider>
    </BrowserRouter>
  );
}
