import { useEffect, useMemo, useState } from "react";
import { Link, Outlet, useLocation, useNavigate, useParams } from "react-router-dom";
import {
  getActiveNavigationSection,
  getDashboardHomeNavigationItem,
  getDashboardNavigationItems,
  getDashboardSidebarNavigationSections,
  isNavigationItemActive,
} from "../app/navigation";
import { appRoutes, buildGuildScopedPath } from "../app/routes";
import {
  buildGuildIconURL,
  formatAuthStateLabel,
  formatSessionTitle,
  getInitials,
} from "../app/utils";
import { useDashboardSession } from "../context/DashboardSessionContext";
import {
  AlertBanner,
  IdentityAvatar,
  PageContentSurface,
  StatusBadge,
} from "../components/ui";
import "../shell.css";

const siteBrandIconSrc = `${import.meta.env.BASE_URL}brand/alicebot.webp`;

export function DashboardLayout() {
  const location = useLocation();
  const navigate = useNavigate();
  const { guildId } = useParams();
  const routeGuildID = guildId?.trim() ?? "";
  const {
    accessibleGuilds,
    authState,
    beginLogin,
    busyLabel,
    manageableGuilds,
    notice,
    selectedGuild,
    selectedGuildID,
    session,
    sessionAvatarURL,
    sessionLoading,
    setSelectedGuildID,
    logout,
  } = useDashboardSession();
  const serverMenuGuilds =
    accessibleGuilds.length > 0 ? accessibleGuilds : manageableGuilds;
  const availableGuildIDs = useMemo(
    () =>
      new Set([
        ...accessibleGuilds.map((guild) => guild.id),
        ...manageableGuilds.map((guild) => guild.id),
      ]),
    [accessibleGuilds, manageableGuilds],
  );
  const navigationSections = useMemo(
    () =>
      routeGuildID === ""
        ? []
        : getDashboardSidebarNavigationSections(routeGuildID),
    [routeGuildID],
  );
  const homeNavigationItem =
    routeGuildID === "" ? null : getDashboardHomeNavigationItem(routeGuildID);
  const activeSection =
    routeGuildID === ""
      ? null
      : getActiveNavigationSection(location.pathname, routeGuildID);
  const navigationItems =
    routeGuildID === "" ? [] : getDashboardNavigationItems(routeGuildID);
  const [accountMenuOpen, setAccountMenuOpen] = useState(false);
  const [serverMenuOpen, setServerMenuOpen] = useState(false);
  const [openSectionID, setOpenSectionID] = useState<string | null>(
    activeSection?.id ?? null,
  );
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  useEffect(() => {
    setAccountMenuOpen(false);
    setServerMenuOpen(false);
  }, [location.pathname, location.search, location.hash, authState]);

  useEffect(() => {
    setOpenSectionID(activeSection?.id ?? null);
  }, [activeSection?.id, location.pathname]);

  useEffect(() => {
    if (authState === "signed_in") {
      return;
    }
    setSelectedGuildID(routeGuildID);
  }, [authState, routeGuildID, setSelectedGuildID]);

  useEffect(() => {
    if (authState !== "signed_in" || routeGuildID === "" || sessionLoading) {
      return;
    }
    if (availableGuildIDs.has(routeGuildID)) {
      return;
    }

    setServerMenuOpen(false);
    setSelectedGuildID("");
    navigate(appRoutes.manage, { replace: true });
  }, [
    authState,
    availableGuildIDs,
    navigate,
    routeGuildID,
    sessionLoading,
    setSelectedGuildID,
  ]);

  async function handleSignOut() {
    setAccountMenuOpen(false);
    await logout();
    navigate(appRoutes.manage);
  }

  async function handleSignIn() {
    setAccountMenuOpen(false);
    await beginLogin(getNextPath(location));
  }

  function handleSelectGuild(nextGuildID: string) {
    const normalizedGuildID = nextGuildID.trim();
    if (normalizedGuildID === "") {
      return;
    }

    setServerMenuOpen(false);
    setSelectedGuildID(normalizedGuildID);
    navigate(buildGuildScopedPath(normalizedGuildID, location.pathname));
  }

  const accountTitle =
    session !== null ? formatSessionTitle(session) : formatAuthStateLabel(authState);
  const accountSubtitle =
    authState === "signed_in"
      ? "Open account menu"
      : authState === "oauth_unavailable"
        ? "OAuth unavailable"
        : "Sign in to continue";
  const showSessionHydrationState = authState === "checking";
  const currentContextLabel = getDashboardContextLabel(
    location.pathname,
    routeGuildID,
    navigationItems,
    homeNavigationItem,
  );
  const sidebarToggleLabel = sidebarCollapsed
    ? "Expand navigation"
    : "Collapse navigation";
  const selectedGuildIconURL =
    selectedGuild !== null ? buildGuildIconURL(selectedGuild) : null;
  const serverTriggerTitle =
    authState !== "signed_in"
      ? "Sign in to view servers"
      : selectedGuild !== null
        ? selectedGuild.name
        : "Select server";
  const serverTriggerSubtitle =
    authState !== "signed_in"
      ? "Administrative servers"
      : selectedGuild !== null
        ? selectedGuild.bot_present === false
          ? "Bot not connected"
          : selectedGuild.access_level === "read"
            ? "Read-only access"
            : "Change server"
        : "Choose workspace";

  function toggleSection(sectionID: string) {
    setOpenSectionID((current) => (current === sectionID ? null : sectionID));
  }

  return (
    <main
      className={`dashboard-layout-shell${
        sidebarCollapsed ? " is-sidebar-collapsed" : ""
      }`}
    >
      <header className="shell-topbar" data-shell-topbar>
        <Link
          className="shell-brand"
          to={appRoutes.manage}
          aria-label="Open manage workspace"
        >
          <span className="shell-brand-mark" aria-hidden="true">
            <img src={siteBrandIconSrc} alt="" />
          </span>
        </Link>

        <div className="shell-topbar-spacer" aria-hidden="true" />

        <div className="shell-server-menu">
          <span className="shell-field-label">Server</span>
          <button
            className="shell-server-trigger"
            type="button"
            aria-label="Server"
            aria-haspopup="menu"
            aria-expanded={serverMenuOpen}
            aria-controls="shell-server-menu"
            disabled={authState !== "signed_in" || serverMenuGuilds.length === 0}
            onClick={() => setServerMenuOpen((current) => !current)}
          >
            {selectedGuild !== null && selectedGuildIconURL ? (
              <span className="shell-server-trigger-mark" aria-hidden="true">
                <img src={selectedGuildIconURL} alt="" />
              </span>
            ) : selectedGuild !== null && selectedGuild.bot_present !== false ? (
              <span className="shell-server-trigger-mark" aria-hidden="true">
                {getInitials(selectedGuild.name)}
              </span>
            ) : null}
            <span className="shell-server-trigger-copy">
              <strong>{serverTriggerTitle}</strong>
              <span>{serverTriggerSubtitle}</span>
            </span>
            <span className="shell-account-trigger-caret" aria-hidden="true">
              v
            </span>
          </button>

          {serverMenuOpen ? (
            <div className="shell-server-menu-panel" id="shell-server-menu" role="menu">
              {serverMenuGuilds.map((guild) => {
                const guildIconURL = buildGuildIconURL(guild);
                const isActive = guild.id === selectedGuildID;

                return (
                  <button
                    className={`shell-server-option${isActive ? " is-active" : ""}`}
                    key={guild.id}
                    type="button"
                    role="menuitem"
                    onClick={() => handleSelectGuild(guild.id)}
                  >
                    {guild.bot_present !== false ? (
                      guildIconURL ? (
                        <span className="shell-server-option-mark" aria-hidden="true">
                          <img src={guildIconURL} alt="" />
                        </span>
                      ) : (
                        <span className="shell-server-option-mark" aria-hidden="true">
                          {getInitials(guild.name)}
                        </span>
                      )
                    ) : null}
                    <span className="shell-server-option-copy">
                      <strong>{guild.name}</strong>
                      <span>
                        {guild.bot_present === false
                          ? "Bot not connected"
                          : guild.access_level === "read"
                            ? "Read-only access"
                            : guild.owner
                            ? "Owner access"
                            : "Administrative access"}
                      </span>
                    </span>
                  </button>
                );
              })}
            </div>
          ) : null}
        </div>

        <div className="shell-account">
          <button
            className="shell-account-trigger"
            type="button"
            aria-haspopup="menu"
            aria-expanded={accountMenuOpen}
            aria-controls="shell-account-menu"
            disabled={sessionLoading}
            onClick={() => setAccountMenuOpen((current) => !current)}
          >
            <IdentityAvatar imageUrl={sessionAvatarURL} label={accountTitle} />
            <span className="shell-account-trigger-copy">
              <strong>{accountTitle}</strong>
              <span>{accountSubtitle}</span>
            </span>
            <span className="shell-account-trigger-caret" aria-hidden="true">
              v
            </span>
          </button>

          {accountMenuOpen ? (
            <div className="shell-account-menu" id="shell-account-menu">
              <div className="shell-account-menu-header">
                <strong>{accountTitle}</strong>
                <p className="shell-account-menu-note">
                  {authState === "signed_in"
                    ? "Use Discord sign-out here when you need to switch accounts."
                    : "Sign in with Discord to load the servers you can manage."}
                </p>
              </div>
              <div className="shell-account-menu-actions">
                {authState === "signed_in" ? (
                  <button
                    className="button-secondary"
                    type="button"
                    disabled={sessionLoading}
                    onClick={() => void handleSignOut()}
                  >
                    Sign out
                  </button>
                ) : (
                  <button
                    className="button-primary"
                    type="button"
                    disabled={sessionLoading}
                    onClick={() => void handleSignIn()}
                  >
                    Sign in with Discord
                  </button>
                )}
              </div>
            </div>
          ) : null}
        </div>
      </header>

      <div className="shell-context-strip" aria-label="Dashboard chrome">
        <div className="shell-context-pane shell-context-pane-nav">
          <span className="shell-context-label">Navigation</span>
          <button
            className="shell-sidebar-toggle button-ghost"
            type="button"
            aria-controls="dashboard-layout-sidebar"
            aria-expanded={!sidebarCollapsed}
            aria-label={sidebarToggleLabel}
            title={sidebarToggleLabel}
            onClick={() => setSidebarCollapsed((current) => !current)}
          >
            <span className="sr-only">{sidebarToggleLabel}</span>
            <span className="shell-sidebar-toggle-bars" aria-hidden="true">
              <span className="shell-sidebar-toggle-line" />
              <span className="shell-sidebar-toggle-line" />
              <span className="shell-sidebar-toggle-line" />
            </span>
          </button>
        </div>

        <div className="shell-context-pane shell-context-pane-content">
          <span className="shell-context-tab shell-context-tab-active">
            {currentContextLabel}
          </span>
        </div>
      </div>

      <div className="shell-body">
        <aside
          className={`dashboard-layout-sidebar${
            sidebarCollapsed ? " is-collapsed" : ""
          }`}
          id="dashboard-layout-sidebar"
          aria-label="Dashboard navigation"
        >
          {homeNavigationItem !== null ? (
            <nav
              className="shell-nav"
              aria-label="Dashboard navigation"
              hidden={sidebarCollapsed}
            >
              <Link
                className={`shell-nav-link shell-nav-link-root${
                  isNavigationItemActive(location.pathname, homeNavigationItem)
                    ? " is-active"
                    : ""
                }`}
                to={homeNavigationItem.to}
                aria-current={
                  isNavigationItemActive(location.pathname, homeNavigationItem)
                    ? "page"
                    : undefined
                }
              >
                <span>{homeNavigationItem.label}</span>
              </Link>

              {navigationSections.map((section) => {
                const hasActiveItem = activeSection?.id === section.id;
                const isOpen =
                  openSectionID === null
                    ? hasActiveItem
                    : openSectionID === section.id;

                return (
                  <section className="shell-nav-section" key={section.id}>
                    <button
                      className={`shell-nav-section-trigger${
                        hasActiveItem ? " is-active" : ""
                      }`}
                      type="button"
                      aria-expanded={isOpen}
                      aria-controls={`shell-nav-section-${section.id}`}
                      onClick={() => toggleSection(section.id)}
                    >
                      <span>{section.label}</span>
                      <span
                        className={`shell-nav-section-indicator${
                          isOpen ? " is-open" : ""
                        }`}
                        aria-hidden="true"
                      >
                        v
                      </span>
                    </button>

                    {isOpen ? (
                      <div
                        className="shell-nav-list"
                        id={`shell-nav-section-${section.id}`}
                      >
                        {section.items.map((item) => {
                          const isActive = isNavigationItemActive(
                            location.pathname,
                            item,
                          );

                          return (
                            <Link
                              key={item.id}
                              className={`shell-nav-link shell-nav-link-sub${
                                isActive ? " is-active" : ""
                              }`}
                              to={item.to}
                              aria-current={isActive ? "page" : undefined}
                            >
                              <span>{item.label}</span>
                            </Link>
                          );
                        })}
                      </div>
                    ) : null}
                  </section>
                );
              })}
            </nav>
          ) : null}
        </aside>

        <section className="shell-main">
          {notice ? (
            <div className="shell-main-notice">
              <AlertBanner
                notice={notice}
                busyLabel={sessionLoading ? busyLabel : undefined}
              />
            </div>
          ) : sessionLoading && !showSessionHydrationState ? (
            <div className="shell-main-notice">
              <AlertBanner busyLabel={busyLabel} />
            </div>
          ) : null}
          {showSessionHydrationState ? (
            <PageContentSurface aria-busy="true">
              <div className="workspace-state">
                <div className="card-copy">
                  <p className="section-label">Dashboard</p>
                  <h2>Loading dashboard</h2>
                  <p className="section-description">
                    Checking your Discord session and preparing the selected
                    server workspace before the page content renders.
                  </p>
                </div>

                <div className="workspace-state-actions">
                  <StatusBadge tone="info">
                    {busyLabel || "Preparing dashboard"}
                  </StatusBadge>
                </div>
              </div>
            </PageContentSurface>
          ) : (
            <Outlet />
          )}
        </section>
      </div>
    </main>
  );
}

function getNextPath(location: ReturnType<typeof useLocation>) {
  return `${location.pathname}${location.search}${location.hash}`;
}

function getDashboardContextLabel(
  pathname: string,
  guildId: string,
  navigationItems: ReturnType<typeof getDashboardNavigationItems>,
  homeNavigationItem: ReturnType<typeof getDashboardHomeNavigationItem> | null,
) {
  if (guildId === "") {
    return "Manage";
  }

  if (homeNavigationItem !== null && isNavigationItemActive(pathname, homeNavigationItem)) {
    return "Home";
  }

  const activeItem = navigationItems.find((item) =>
    isNavigationItemActive(pathname, item),
  );

  return activeItem?.label ?? "Workspace";
}
