import { useState } from "react";
import { Link, Navigate, useLocation, useParams } from "react-router-dom";
import type { FeatureRecord } from "../api/control";
import { appRoutes } from "../app/routes";
import { useDashboardSession } from "../context/DashboardSessionContext";
import {
  getFeatureAreaDefinition,
  getFeatureAreaRecords,
  type FeatureAreaID,
} from "../features/features/areas";
import {
  isFeatureBlocked,
  isFeatureConfigurable,
} from "../features/features/model";
import {
  formatEffectiveSourceLabel,
  formatFeatureSignal,
  formatFeatureSignalTitle,
  formatFeatureStatusLabel,
  formatFeatureStatusSupport,
  formatOverrideLabel,
  formatWorkspaceStateDescription,
  formatWorkspaceStateTitle,
  getFeatureStatusTone,
  summarizeFeatureArea,
} from "../features/features/presentation";
import { useFeatureMutation } from "../features/features/useFeatureMutation";
import { useFeatureWorkspace } from "../features/features/useFeatureWorkspace";
import {
  AlertBanner,
  EmptyState,
  KeyValueList,
  MetricCard,
  PageHeader,
  StatusBadge,
  SurfaceCard,
} from "../components/ui";

interface FeatureCategoryPageProps {
  areaId?: FeatureAreaID;
}

export function FeatureCategoryPage({ areaId }: FeatureCategoryPageProps) {
  const location = useLocation();
  const params = useParams();
  const resolvedAreaId = (areaId ?? params.areaId ?? "") as FeatureAreaID | "";
  const definition =
    resolvedAreaId === "" ? null : getFeatureAreaDefinition(resolvedAreaId);
  const { authState, beginLogin, currentOriginLabel, selectedGuild } =
    useDashboardSession();
  const workspace = useFeatureWorkspace({
    scope: "guild",
  });
  const mutation = useFeatureMutation({
    scope: "guild",
  });
  const [pendingFeatureId, setPendingFeatureId] = useState("");

  const dedicatedRoute = getDedicatedAreaRoute(resolvedAreaId);
  if (dedicatedRoute !== null) {
    return <Navigate replace to={dedicatedRoute} />;
  }

  if (definition === null) {
    return <Navigate replace to={appRoutes.dashboardHome} />;
  }

  const areaLabel = definition.label;
  const areaDescription = definition.description;
  const nextPath = `${location.pathname}${location.search}${location.hash}`;
  const areaFeatures = getFeatureAreaRecords(workspace.features, definition.id);
  const areaSummary = summarizeFeatureArea(areaFeatures);
  const blockingFeature =
    areaFeatures.find((feature) => isFeatureBlocked(feature)) ?? null;
  const workspaceNotice = mutation.notice ?? workspace.notice;
  const selectedServerLabel = selectedGuild?.name ?? "No server selected";
  const localOverrides = areaFeatures.filter(
    (feature) => feature.override_state !== "inherit",
  ).length;
  const configurableFeatures = areaFeatures.filter((feature) =>
    isFeatureConfigurable(feature),
  ).length;

  async function handleRefreshArea() {
    await workspace.refresh();
  }

  async function handleSetFeatureEnabled(
    feature: FeatureRecord,
    enabled: boolean,
  ) {
    setPendingFeatureId(feature.id);

    try {
      const updated = await mutation.patchFeature(feature.id, {
        enabled,
      });
      if (updated !== null) {
        await workspace.refresh();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleUseInherited(feature: FeatureRecord) {
    setPendingFeatureId(feature.id);

    try {
      const updated = await mutation.patchFeature(feature.id, {
        enabled: null,
      });
      if (updated !== null) {
        await workspace.refresh();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  function renderHeaderActions() {
    if (authState !== "signed_in") {
      return (
        <button
          className="button-primary"
          type="button"
          onClick={() => void beginLogin(nextPath)}
        >
          Sign in with Discord
        </button>
      );
    }

    if (selectedGuild === null) {
      return null;
    }

    return (
      <button
        className="button-secondary"
        type="button"
        disabled={workspace.loading || mutation.saving}
        onClick={() => void handleRefreshArea()}
      >
        Refresh area
      </button>
    );
  }

  function renderWorkspaceContent() {
    if (workspace.workspaceState !== "ready") {
      return (
        <EmptyState
          title={formatWorkspaceStateTitle(areaLabel, workspace.workspaceState)}
          description={formatWorkspaceStateDescription(
            areaLabel,
            workspace.workspaceState,
          )}
          action={
            authState !== "signed_in" ? (
              <button
                className="button-primary"
                type="button"
                onClick={() => void beginLogin(nextPath)}
              >
                Sign in with Discord
              </button>
            ) : workspace.workspaceState === "unavailable" ? (
              <button
                className="button-secondary"
                type="button"
                onClick={() => void handleRefreshArea()}
              >
                Retry loading
              </button>
            ) : undefined
          }
        />
      );
    }

    if (areaFeatures.length === 0) {
      return (
        <div className="table-empty-state table-empty-state-compact">
          <p className="section-label">Workspace</p>
          <h2>No mapped features yet</h2>
          <p>
            This category does not have feature records mapped for the selected
            server yet.
          </p>
        </div>
      );
    }

    return (
      <div className="table-wrap">
        <table className="data-table feature-table">
          <thead>
            <tr>
              <th scope="col">Feature</th>
              <th scope="col">Status</th>
              <th scope="col">Inheritance</th>
              <th scope="col">Signal</th>
              <th scope="col">Actions</th>
            </tr>
          </thead>
          <tbody>
            {areaFeatures.map((feature) => {
              const isPending =
                mutation.saving && pendingFeatureId === feature.id;

              return (
                <tr key={feature.id}>
                  <td>
                    <div className="feature-table-copy">
                      <strong>{feature.label}</strong>
                      <p>{feature.description}</p>
                      {isFeatureConfigurable(feature) ? (
                        <span className="meta-note">
                          Additional settings exist for this feature beyond the
                          enabled state.
                        </span>
                      ) : null}
                    </div>
                  </td>
                  <td>
                    <div className="feature-status-cell">
                      <StatusBadge tone={getFeatureStatusTone(feature)}>
                        {formatFeatureStatusLabel(feature)}
                      </StatusBadge>
                      <span className="meta-note">
                        {formatFeatureStatusSupport(feature)}
                      </span>
                    </div>
                  </td>
                  <td>
                    <div className="feature-table-meta">
                      <strong>
                        {formatOverrideLabel(feature.override_state)}
                      </strong>
                      <span className="meta-note">
                        {formatEffectiveSourceLabel(feature.effective_source)}
                      </span>
                    </div>
                  </td>
                  <td>
                    <div className="feature-table-copy">
                      <strong>{formatFeatureSignalTitle(feature)}</strong>
                      <p>{formatFeatureSignal(feature)}</p>
                    </div>
                  </td>
                  <td>
                    <div className="feature-row-actions">
                      {feature.supports_guild_override ? (
                        <>
                          <button
                            className="button-secondary"
                            type="button"
                            disabled={mutation.saving}
                            aria-label={`${feature.effective_enabled ? "Disable" : "Enable"} ${feature.label}`}
                            onClick={() =>
                              void handleSetFeatureEnabled(
                                feature,
                                !feature.effective_enabled,
                              )
                            }
                          >
                            {isPending
                              ? "Saving..."
                              : feature.effective_enabled
                                ? "Disable"
                                : "Enable"}
                          </button>
                          {feature.override_state !== "inherit" ? (
                            <button
                              className="button-ghost"
                              type="button"
                              disabled={mutation.saving}
                              aria-label={`Use inherited setting for ${feature.label}`}
                              onClick={() => void handleUseInherited(feature)}
                            >
                              Use inherited
                            </button>
                          ) : null}
                        </>
                      ) : (
                        <span className="meta-note">
                          Managed outside this server workspace.
                        </span>
                      )}
                    </div>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    );
  }

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Fallback workspace"
        title={areaLabel}
        description={`${areaDescription} This generic feature workspace is kept only for areas that do not have a dedicated operator page yet.`}
        status={
          <StatusBadge
            tone={
              workspace.workspaceState === "ready" ? areaSummary.tone : "info"
            }
          >
            {workspace.workspaceState === "ready"
              ? areaSummary.label
              : formatWorkspaceStateTitle(areaLabel, workspace.workspaceState)}
          </StatusBadge>
        }
        meta={
          <>
            <span className="meta-pill subtle-pill">{selectedServerLabel}</span>
            <span className="meta-pill subtle-pill">{currentOriginLabel}</span>
          </>
        }
        actions={renderHeaderActions()}
      />

      {workspace.workspaceState === "ready" ? (
        <section
          className="overview-summary-strip"
          aria-label={`${areaLabel} summary`}
        >
          <MetricCard
            label="Tracked features"
            value={String(areaSummary.total)}
            description="Features currently mapped into this category."
          />
          <MetricCard
            label="Ready"
            value={String(areaSummary.ready)}
            description="Features that are enabled and not reporting blockers."
            tone={areaSummary.ready > 0 ? "success" : "neutral"}
          />
          <MetricCard
            label="Needs attention"
            value={String(areaSummary.blocked)}
            description="Enabled features that still report a blocker."
            tone={areaSummary.blocked > 0 ? "error" : "neutral"}
          />
          <MetricCard
            label="Disabled"
            value={String(areaSummary.disabled)}
            description="Features currently turned off for this server."
          />
        </section>
      ) : null}

      <section className="content-grid content-grid-with-aside">
        <div className="page-main">
          <SurfaceCard className="feature-category-panel">
            <div className="workspace-view">
              <div className="workspace-view-header">
                <div className="card-copy">
                  <p className="section-label">Fallback</p>
                  <h2>Generic feature list</h2>
                  <p className="section-description">
                    Use this page only for areas that still rely on the generic
                    feature table. Dedicated workspaces should be preferred
                    whenever they exist.
                  </p>
                </div>
                <div className="workspace-view-meta">
                  {workspace.workspaceState === "ready" ? (
                    <>
                      <span className="meta-pill subtle-pill">
                        {localOverrides} local overrides
                      </span>
                      <span className="meta-pill subtle-pill">
                        {configurableFeatures} with extra settings
                      </span>
                    </>
                  ) : null}
                </div>
              </div>

              <AlertBanner
                notice={workspaceNotice}
                busyLabel={
                  mutation.saving
                    ? "Saving feature state..."
                    : workspace.loading
                      ? "Refreshing feature workspace..."
                      : undefined
                }
              />

              {renderWorkspaceContent()}
            </div>
          </SurfaceCard>
        </div>

        <aside className="page-aside">
          <SurfaceCard>
            <div className="card-copy">
              <p className="section-label">Summary</p>
              <h2>Fallback health</h2>
              <p className="section-description">
                Keep this page narrowly focused on feature state for the few
                remaining areas that have not been specialized yet.
              </p>
            </div>

            <KeyValueList
              items={[
                {
                  label: "Server",
                  value: selectedServerLabel,
                },
                {
                  label: "Local overrides",
                  value: String(localOverrides),
                },
                {
                  label: "Extra settings",
                  value: String(configurableFeatures),
                },
                {
                  label: "Current signal",
                  value:
                    workspace.workspaceState === "ready"
                      ? areaSummary.signal
                      : formatWorkspaceStateTitle(
                          areaLabel,
                          workspace.workspaceState,
                        ),
                },
              ]}
            />
          </SurfaceCard>

          <SurfaceCard>
            <div className="card-copy">
              <p className="section-label">Guidance</p>
              <h2>Why this page exists</h2>
              <p className="section-description">
                This generic workspace is a fallback. The main modules now use
                dedicated pages so operators do not have to manage them from a
                raw feature table.
              </p>
            </div>

            <ul className="feature-guidance-list">
              <li>
                Enable or disable a feature without leaving the category list.
              </li>
              <li>
                Use inherited to clear the server override and fall back to the
                configured default.
              </li>
              <li>
                Readiness and blockers always come from the control server, not
                from client-side rules.
              </li>
            </ul>

            {blockingFeature ? (
              <div className="surface-subsection">
                <p className="section-label">Current blocker</p>
                <strong>{blockingFeature.label}</strong>
                <p className="meta-note">
                  {blockingFeature.blockers?.[0]?.message ?? areaSummary.signal}
                </p>
              </div>
            ) : null}

            <div className="surface-subsection">
              <p className="section-label">Dedicated workspaces</p>
              <p className="meta-note">
                Commands, moderation, logging, roles, stats, Partner Board, and
                advanced maintenance already have dedicated operator pages.
              </p>
              <div className="feature-row-actions">
                <Link className="button-secondary" to={appRoutes.dashboardHome}>
                  Return home
                </Link>
              </div>
            </div>
          </SurfaceCard>
        </aside>
      </section>
    </section>
  );
}

function getDedicatedAreaRoute(areaId: FeatureAreaID | "") {
  switch (areaId) {
    case "commands":
      return appRoutes.commands;
    case "moderation":
      return appRoutes.moderation;
    case "logging":
      return appRoutes.logging;
    case "roles":
      return appRoutes.roles;
    case "stats":
      return appRoutes.stats;
    case "maintenance":
      return appRoutes.settingsAdvanced;
    default:
      return null;
  }
}
