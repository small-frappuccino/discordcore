import { useLocation } from "react-router-dom";
import { EmptyState, PageContentSurface } from "../components/ui";
import { useDashboardSession } from "../context/DashboardSessionContext";

export function ManageIndexPage() {
  const location = useLocation();
  const { authState, beginLogin, manageableGuilds, sessionLoading } =
    useDashboardSession();

  return (
    <section className="page-shell">
      <PageContentSurface>
        <EmptyState
          title={authState === "signed_in" ? "Select a server" : "Sign in to continue"}
          description={
            authState === "signed_in"
              ? manageableGuilds.length > 0
                ? "Use the server menu in the top bar to open a workspace."
                : "No administrative servers are available for this account yet."
              : "Use Discord sign-in to load the servers you can manage from this control panel."
          }
          action={
            authState !== "signed_in" ? (
              <button
                className="button-primary"
                type="button"
                disabled={sessionLoading}
                onClick={() =>
                  void beginLogin(`${location.pathname}${location.search}${location.hash}`)
                }
              >
                Sign in with Discord
              </button>
            ) : undefined
          }
        />
      </PageContentSurface>
    </section>
  );
}
