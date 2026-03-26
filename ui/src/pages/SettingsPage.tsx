import { useEffect, useState } from "react";
import { useLocation } from "react-router-dom";
import {
  AlertBanner,
  EntityMultiPickerField,
  EntityPickerField,
  EmptyState,
  KeyValueList,
  MetricCard,
  PageHeader,
  StatusBadge,
  SurfaceCard,
} from "../components/ui";
import type { FeatureRecord } from "../api/control";
import { useDashboardSession } from "../context/DashboardSessionContext";
import {
  formatAuthStateLabel,
  formatAuthSupportText,
  formatSessionTitle,
  formatTimestamp,
} from "../app/utils";
import type { Notice, SettingsNavigationState } from "../app/types";
import { getAdvancedFeatureRecords } from "../features/features/areas";
import {
  buildMessageRouteChannelPickerOptions,
  formatGuildChannelValue,
} from "../features/features/discordEntities";
import {
  canEditBackfill,
  canEditUserPrune,
  formatBackfillScheduleValue,
  formatUserPruneExemptRoleCountValue,
  formatUserPruneExemptRolesValue,
  formatUserPruneRoleOptionLabel,
  formatUserPruneRuleValue,
  formatUserPruneRunModeValue,
  getBackfillFeatureDetails,
  getUserPruneFeatureDetails,
  summarizeBackfillSignal,
  summarizeUserPruneSignal,
  toggleExemptRole,
} from "../features/features/maintenance";
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
  buildDeliveryPayload,
  formsFromBoard,
  postingMethodLabel,
  validateDeliveryForm,
  type DeliveryFormState,
} from "../features/partner-board/model";
import { usePartnerBoardSummary } from "../features/partner-board/usePartnerBoardSummary";
import { useGuildChannelOptions } from "../features/features/useGuildChannelOptions";
import { useGuildRoleOptions } from "../features/features/useGuildRoleOptions";

type SettingsSection = "server" | "connection" | "permissions" | "advanced";

const settingsSections: Array<{
  id: SettingsSection;
  label: string;
}> = [
  {
    id: "server",
    label: "Server",
  },
  {
    id: "connection",
    label: "Connection",
  },
  {
    id: "permissions",
    label: "Permissions",
  },
  {
    id: "advanced",
    label: "Advanced",
  },
];

const initialDiagnosticsDeliveryForm: DeliveryFormState = {
  type: "channel_message",
  messageID: "",
  webhookURL: "",
  channelID: "",
};

export function SettingsPage() {
  const location = useLocation();
  const navigationState = (location.state ??
    null) as SettingsNavigationState | null;
  const requestedTargetType =
    navigationState?.diagnostics?.partnerBoardTargetType;
  const diagnosticsRequested =
    location.hash === "#diagnostics" || requestedTargetType !== undefined;
  const activeSection = resolveSettingsSection(location.hash);
  const [deliveryForm, setDeliveryForm] = useState<DeliveryFormState>(
    initialDiagnosticsDeliveryForm,
  );
  const [diagnosticsNotice, setDiagnosticsNotice] = useState<Notice | null>(
    null,
  );
  const [diagnosticsOpen, setDiagnosticsOpen] = useState(diagnosticsRequested);
  const [pendingFeatureId, setPendingFeatureId] = useState("");
  const [selectedAdvancedFeatureID, setSelectedAdvancedFeatureID] =
    useState("");
  const [backfillChannelDraft, setBackfillChannelDraft] = useState("");
  const [backfillStartDayDraft, setBackfillStartDayDraft] = useState("");
  const [backfillInitialDateDraft, setBackfillInitialDateDraft] = useState("");
  const [pruneConfigEnabledDraft, setPruneConfigEnabledDraft] =
    useState("enabled");
  const [pruneGraceDaysDraft, setPruneGraceDaysDraft] = useState("");
  const [pruneScanIntervalDraft, setPruneScanIntervalDraft] = useState("");
  const [pruneInitialDelayDraft, setPruneInitialDelayDraft] = useState("");
  const [pruneKicksPerSecondDraft, setPruneKicksPerSecondDraft] = useState("");
  const [pruneMaxKicksPerRunDraft, setPruneMaxKicksPerRunDraft] = useState("");
  const [pruneExemptRoleIDsDraft, setPruneExemptRoleIDsDraft] = useState<
    string[]
  >([]);
  const [pruneRunModeDraft, setPruneRunModeDraft] = useState("simulation");
  const [savingDelivery, setSavingDelivery] = useState(false);
  const {
    applyBaseUrl,
    authState,
    baseUrlDirty,
    baseUrlDraft,
    beginLogin,
    client,
    currentOriginLabel,
    manageableGuilds,
    selectedGuild,
    selectedGuildID,
    session,
    sessionLoading,
    setBaseUrlDraft,
  } = useDashboardSession();
  const featureWorkspace = useFeatureWorkspace({
    scope: "guild",
  });
  const featureMutation = useFeatureMutation({
    scope: "guild",
  });
  const channelOptions = useGuildChannelOptions();
  const roleOptions = useGuildRoleOptions();
  const {
    board,
    deliveryConfigured,
    lastLoadedAt,
    loading: boardSummaryLoading,
    notice: boardSummaryNotice,
    postingMethodLabel: boardPostingMethodLabel,
    refreshBoardSummary,
    shellStatus,
    summarizePostingDestination,
  } = usePartnerBoardSummary();
  const nextPath = `${location.pathname}${location.search}${location.hash}`;

  useEffect(() => {
    if (!diagnosticsRequested) {
      return;
    }

    setDiagnosticsOpen(true);

    requestAnimationFrame(() => {
      const diagnosticsSection = document.getElementById("diagnostics");
      if (
        diagnosticsSection &&
        typeof diagnosticsSection.scrollIntoView === "function"
      ) {
        diagnosticsSection.scrollIntoView({
          block: "start",
        });
      }
    });
  }, [diagnosticsRequested]);

  useEffect(() => {
    const nextDeliveryForm = formsFromBoard(board).deliveryForm;
    if (
      requestedTargetType === "channel_message" ||
      requestedTargetType === "webhook_message"
    ) {
      setDeliveryForm({
        ...nextDeliveryForm,
        type: requestedTargetType,
      });
      return;
    }

    setDeliveryForm(nextDeliveryForm);
  }, [board, requestedTargetType]);

  async function handleSaveDelivery() {
    if (authState !== "signed_in" || selectedGuild === null) {
      setDiagnosticsNotice({
        tone: "info",
        message:
          "Sign in and select a server before editing Partner Board diagnostics.",
      });
      return;
    }

    const validationError = validateDeliveryForm(deliveryForm);
    if (validationError !== null) {
      setDiagnosticsNotice({
        tone: "error",
        message: validationError,
      });
      return;
    }

    setSavingDelivery(true);

    try {
      await client.setPartnerBoardTarget(
        selectedGuild.id,
        buildDeliveryPayload(deliveryForm),
      );
      await refreshBoardSummary();
      setDiagnosticsNotice({
        tone: "success",
        message: "Partner Board destination updated.",
      });
    } catch (error) {
      setDiagnosticsNotice({
        tone: "error",
        message: error instanceof Error ? error.message : String(error),
      });
    } finally {
      setSavingDelivery(false);
    }
  }

  const diagnosticsNoticeToRender = diagnosticsNotice ?? boardSummaryNotice;
  const advancedNotice = featureMutation.notice ?? featureWorkspace.notice;
  const boardReadyLabel =
    selectedGuild === null ? "No server selected" : shellStatus.label;
  const accountLabel =
    session !== null
      ? formatSessionTitle(session)
      : formatAuthStateLabel(authState);
  const authSupportText = formatAuthSupportText(
    authState,
    manageableGuilds.length,
  );
  const advancedFeatures = getAdvancedFeatureRecords(featureWorkspace.features);
  const backfillFeature =
    advancedFeatures.find((feature) => feature.id === "backfill.enabled") ??
    null;
  const userPruneFeature =
    advancedFeatures.find((feature) => feature.id === "user_prune") ?? null;
  const supportingAdvancedFeatures = advancedFeatures.filter(
    (feature) =>
      feature.id !== "backfill.enabled" && feature.id !== "user_prune",
  );
  const selectedAdvancedFeature =
    selectedAdvancedFeatureID === ""
      ? null
      : (advancedFeatures.find(
          (feature) => feature.id === selectedAdvancedFeatureID,
        ) ?? null);
  const backfillDetails =
    backfillFeature === null
      ? null
      : getBackfillFeatureDetails(backfillFeature);
  const userPruneDetails =
    userPruneFeature === null
      ? null
      : getUserPruneFeatureDetails(userPruneFeature);
  const advancedSummary = summarizeFeatureArea(advancedFeatures);
  const advancedBlockingFeature =
    advancedFeatures.find((feature) => isFeatureBlocked(feature)) ?? null;
  const advancedOverrides = advancedFeatures.filter(
    (feature) => feature.override_state !== "inherit",
  ).length;
  const advancedConfigurable = advancedFeatures.filter((feature) =>
    isFeatureConfigurable(feature),
  ).length;
  const diagnosticsBusyLabel = savingDelivery
    ? "Saving destination"
    : boardSummaryLoading
      ? "Refreshing diagnostics"
      : undefined;
  const draftModeLabel = baseUrlDirty ? "Unsaved changes" : "Saved";
  const backfillChannelPickerOptions = buildMessageRouteChannelPickerOptions(
    channelOptions.channels,
  );
  const parsedPruneGraceDays = Number.parseInt(pruneGraceDaysDraft.trim(), 10);
  const parsedPruneScanIntervalMins = Number.parseInt(
    pruneScanIntervalDraft.trim(),
    10,
  );
  const parsedPruneInitialDelaySecs = Number.parseInt(
    pruneInitialDelayDraft.trim(),
    10,
  );
  const parsedPruneKicksPerSecond = Number.parseInt(
    pruneKicksPerSecondDraft.trim(),
    10,
  );
  const parsedPruneMaxKicksPerRun = Number.parseInt(
    pruneMaxKicksPerRunDraft.trim(),
    10,
  );
  const canSaveUserPrune =
    Number.isFinite(parsedPruneGraceDays) &&
    Number.isFinite(parsedPruneScanIntervalMins) &&
    Number.isFinite(parsedPruneInitialDelaySecs) &&
    Number.isFinite(parsedPruneKicksPerSecond) &&
    Number.isFinite(parsedPruneMaxKicksPerRun);

  async function handleRefreshAdvancedControls() {
    await Promise.all([
      featureWorkspace.refresh(),
      channelOptions.refresh(),
      roleOptions.refresh(),
    ]);
  }

  async function handleSetAdvancedFeatureEnabled(
    featureID: string,
    enabled: boolean,
  ) {
    setPendingFeatureId(featureID);

    try {
      const updated = await featureMutation.patchFeature(featureID, {
        enabled,
      });
      if (updated !== null) {
        await featureWorkspace.refresh();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleUseAdvancedDefault(featureID: string) {
    setPendingFeatureId(featureID);

    try {
      const updated = await featureMutation.patchFeature(featureID, {
        enabled: null,
      });
      if (updated !== null) {
        await featureWorkspace.refresh();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  function openAdvancedDrawer(feature: FeatureRecord) {
    featureMutation.clearNotice();

    if (feature.id === "backfill.enabled") {
      const details = getBackfillFeatureDetails(feature);
      setBackfillChannelDraft(details.channelId);
      setBackfillStartDayDraft(details.startDay);
      setBackfillInitialDateDraft(details.initialDate);
      setSelectedAdvancedFeatureID(feature.id);
      return;
    }

    if (feature.id === "user_prune") {
      const details = getUserPruneFeatureDetails(feature);
      setPruneConfigEnabledDraft(
        details.configEnabled ? "enabled" : "disabled",
      );
      setPruneGraceDaysDraft(String(details.graceDays));
      setPruneScanIntervalDraft(String(details.scanIntervalMins));
      setPruneInitialDelayDraft(String(details.initialDelaySecs));
      setPruneKicksPerSecondDraft(String(details.kicksPerSecond));
      setPruneMaxKicksPerRunDraft(String(details.maxKicksPerRun));
      setPruneExemptRoleIDsDraft(details.exemptRoleIds);
      setPruneRunModeDraft(details.dryRun ? "simulation" : "live");
      setSelectedAdvancedFeatureID(feature.id);
    }
  }

  function closeAdvancedDrawer() {
    setSelectedAdvancedFeatureID("");
    setBackfillChannelDraft("");
    setBackfillStartDayDraft("");
    setBackfillInitialDateDraft("");
    setPruneConfigEnabledDraft("enabled");
    setPruneGraceDaysDraft("");
    setPruneScanIntervalDraft("");
    setPruneInitialDelayDraft("");
    setPruneKicksPerSecondDraft("");
    setPruneMaxKicksPerRunDraft("");
    setPruneExemptRoleIDsDraft([]);
    setPruneRunModeDraft("simulation");
    featureMutation.clearNotice();
  }

  async function handleSaveBackfill() {
    if (backfillFeature === null) {
      return;
    }

    setPendingFeatureId(backfillFeature.id);

    try {
      const updated = await featureMutation.patchFeature(backfillFeature.id, {
        channel_id: backfillChannelDraft.trim(),
        start_day: backfillStartDayDraft.trim(),
        initial_date: backfillInitialDateDraft.trim(),
      });
      if (updated !== null) {
        await featureWorkspace.refresh();
        closeAdvancedDrawer();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  async function handleSaveUserPrune() {
    if (userPruneFeature === null || !canSaveUserPrune) {
      return;
    }

    setPendingFeatureId(userPruneFeature.id);

    try {
      const updated = await featureMutation.patchFeature(userPruneFeature.id, {
        config_enabled: pruneConfigEnabledDraft === "enabled",
        grace_days: parsedPruneGraceDays,
        scan_interval_mins: parsedPruneScanIntervalMins,
        initial_delay_secs: parsedPruneInitialDelaySecs,
        kicks_per_second: parsedPruneKicksPerSecond,
        max_kicks_per_run: parsedPruneMaxKicksPerRun,
        exempt_role_ids: pruneExemptRoleIDsDraft,
        dry_run: pruneRunModeDraft === "simulation",
      });
      if (updated !== null) {
        await featureWorkspace.refresh();
        closeAdvancedDrawer();
      }
    } finally {
      setPendingFeatureId("");
    }
  }

  return (
    <section className="page-shell">
      <PageHeader
        eyebrow="Settings"
        title="Settings"
        description="Keep technical controls and diagnostics separate from the day-to-day feature workspace."
        status={
          <StatusBadge tone={authState === "signed_in" ? "success" : "info"}>
            {formatAuthStateLabel(authState)}
          </StatusBadge>
        }
        meta={
          <>
            <span className="meta-pill subtle-pill">
              {selectedGuild?.name ?? "No server selected"}
            </span>
            <span className="meta-pill subtle-pill">{currentOriginLabel}</span>
          </>
        }
      />

      <SurfaceCard className="workspace-panel settings-workspace-panel">
        <nav
          className="subnav workspace-tabs settings-subnav"
          aria-label="Settings sections"
        >
          {settingsSections.map((section) => (
            <a
              key={section.id}
              className={`subnav-link${activeSection === section.id ? " is-active" : ""}`}
              href={`#${section.id}`}
            >
              {section.label}
            </a>
          ))}
        </nav>

        <div className="workspace-panel-body">
          {activeSection === "server" ? (
            <section className="workspace-view" id="server">
              <div className="workspace-view-header">
                <div className="card-copy">
                  <p className="section-label">Server</p>
                  <h2>Current server</h2>
                  <p className="section-description">
                    Server selection is shared across feature workspaces. Choose
                    it in the sidebar, then manage the active server here.
                  </p>
                </div>
                <div className="workspace-view-meta">
                  <StatusBadge tone={shellStatus.tone}>
                    {boardReadyLabel}
                  </StatusBadge>
                </div>
              </div>

              <div className="settings-summary-strip">
                <MetricCard
                  label="Server"
                  value={selectedGuild?.name ?? "Not selected"}
                  description={
                    selectedGuild === null
                      ? "Choose a server from the sidebar."
                      : "This is the active server for dashboard workspaces."
                  }
                  tone={selectedGuild === null ? "info" : "success"}
                />
                <MetricCard
                  label="Readiness"
                  value={boardReadyLabel}
                  description={shellStatus.description}
                  tone={shellStatus.tone}
                />
                <MetricCard
                  label="Servers"
                  value={String(manageableGuilds.length)}
                  description={authSupportText}
                  tone={manageableGuilds.length > 0 ? "success" : "info"}
                />
                <MetricCard
                  label="Destination"
                  value={deliveryConfigured ? "Configured" : "Incomplete"}
                  description={summarizePostingDestination}
                  tone={deliveryConfigured ? "success" : "info"}
                />
              </div>

              <KeyValueList
                className="workspace-status-list"
                items={[
                  {
                    label: "Selected server",
                    value: selectedGuild?.name ?? "No server selected",
                  },
                  {
                    label: "Partner Board status",
                    value: shellStatus.description,
                  },
                  {
                    label: "Posting method",
                    value: boardPostingMethodLabel,
                  },
                  {
                    label: "Last board check",
                    value: formatTimestamp(lastLoadedAt, "Not checked yet"),
                  },
                ]}
              />
            </section>
          ) : null}

          {activeSection === "connection" ? (
            <section className="workspace-view" id="connection">
              <div className="workspace-view-header">
                <div className="card-copy">
                  <p className="section-label">Connection</p>
                  <h2>Control connection</h2>
                  <p className="section-description">
                    Point the dashboard at a control server only when you
                    intentionally need another backend endpoint.
                  </p>
                </div>
                <div className="workspace-view-meta">
                  <span className="meta-pill subtle-pill">
                    {draftModeLabel}
                  </span>
                </div>
              </div>

              <KeyValueList
                className="workspace-status-list"
                items={[
                  {
                    label: "Saved endpoint",
                    value: currentOriginLabel,
                  },
                  {
                    label: "Draft state",
                    value: baseUrlDirty
                      ? "The draft differs from the saved endpoint."
                      : "The draft matches the saved endpoint.",
                  },
                  {
                    label: "Session status",
                    value: formatAuthStateLabel(authState),
                  },
                ]}
              />

              <label className="field-stack">
                <span className="field-label">Connection URL</span>
                <input
                  value={baseUrlDraft}
                  onChange={(event) => setBaseUrlDraft(event.target.value)}
                  placeholder="Leave blank to use the current origin"
                />
              </label>

              <div className="workspace-footer">
                <button
                  className="button-primary"
                  type="button"
                  disabled={sessionLoading || !baseUrlDirty}
                  onClick={applyBaseUrl}
                >
                  Save connection
                </button>
                <span className="meta-note">
                  Keep this value stable unless the dashboard should target
                  another control server.
                </span>
              </div>
            </section>
          ) : null}

          {activeSection === "permissions" ? (
            <section className="workspace-view" id="permissions">
              <div className="workspace-view-header">
                <div className="card-copy">
                  <p className="section-label">Permissions</p>
                  <h2>Access summary</h2>
                  <p className="section-description">
                    Show the user-facing access state here and keep raw OAuth
                    scopes and identifiers inside Diagnostics.
                  </p>
                </div>
                <div className="workspace-view-meta">
                  <StatusBadge
                    tone={authState === "signed_in" ? "success" : "info"}
                  >
                    {formatAuthStateLabel(authState)}
                  </StatusBadge>
                </div>
              </div>

              <div className="settings-summary-strip settings-summary-strip-compact">
                <MetricCard
                  label="Session"
                  value={
                    authState === "signed_in"
                      ? "Connected"
                      : formatAuthStateLabel(authState)
                  }
                  description={accountLabel}
                  tone={authState === "signed_in" ? "success" : "info"}
                />
                <MetricCard
                  label="Servers"
                  value={String(manageableGuilds.length)}
                  description={authSupportText}
                  tone={manageableGuilds.length > 0 ? "success" : "info"}
                />
                <MetricCard
                  label="Partner Board"
                  value={boardReadyLabel}
                  description={
                    selectedGuild === null
                      ? "Select a server to load feature access."
                      : shellStatus.description
                  }
                  tone={selectedGuild === null ? "info" : shellStatus.tone}
                />
              </div>

              <KeyValueList
                className="workspace-status-list"
                items={[
                  {
                    label: "Account",
                    value: accountLabel,
                  },
                  {
                    label: "Server management access",
                    value: authSupportText,
                  },
                  {
                    label: "Partner Board access",
                    value:
                      selectedGuild === null
                        ? "Select a server to load feature access."
                        : shellStatus.description,
                  },
                ]}
              />
            </section>
          ) : null}

          {activeSection === "advanced" ? (
            <section className="workspace-view" id="advanced">
              <div className="workspace-view-header">
                <div className="card-copy">
                  <p className="section-label">Advanced</p>
                  <h2>Runtime and maintenance controls</h2>
                  <p className="section-description">
                    Cleanup, backfill, cache, and prune routines stay here so
                    the main navigation remains focused on day-to-day operator
                    workspaces.
                  </p>
                </div>
                <div className="workspace-view-meta">
                  <StatusBadge
                    tone={
                      featureWorkspace.workspaceState === "ready"
                        ? advancedSummary.tone
                        : "info"
                    }
                  >
                    {featureWorkspace.workspaceState === "ready"
                      ? advancedSummary.label
                      : formatWorkspaceStateTitle(
                          "Advanced controls",
                          featureWorkspace.workspaceState,
                        )}
                  </StatusBadge>
                </div>
              </div>

              <AlertBanner
                notice={advancedNotice}
                busyLabel={
                  featureMutation.saving
                    ? "Saving advanced controls..."
                    : featureWorkspace.loading
                      ? "Refreshing advanced controls..."
                      : undefined
                }
              />

              {featureWorkspace.workspaceState !== "ready" ? (
                <EmptyState
                  title={formatWorkspaceStateTitle(
                    "Advanced controls",
                    featureWorkspace.workspaceState,
                  )}
                  description={formatWorkspaceStateDescription(
                    "Advanced controls",
                    featureWorkspace.workspaceState,
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
                    ) : featureWorkspace.workspaceState === "unavailable" ? (
                      <button
                        className="button-secondary"
                        type="button"
                        onClick={() => void handleRefreshAdvancedControls()}
                      >
                        Retry loading
                      </button>
                    ) : undefined
                  }
                />
              ) : advancedFeatures.length === 0 ? (
                <div className="table-empty-state table-empty-state-compact">
                  <div className="card-copy">
                    <p className="section-label">Advanced</p>
                    <h2>No advanced controls mapped yet</h2>
                    <p className="section-description">
                      The selected server does not currently expose advanced
                      maintenance or runtime routines in this section.
                    </p>
                  </div>
                </div>
              ) : (
                <>
                  <div className="settings-summary-strip settings-summary-strip-compact">
                    <MetricCard
                      label="Advanced features"
                      value={String(advancedSummary.total)}
                      description="Advanced routines currently surfaced from feature records."
                    />
                    <MetricCard
                      label="Ready"
                      value={String(advancedSummary.ready)}
                      description="Advanced features that are enabled and not reporting blockers."
                      tone={advancedSummary.ready > 0 ? "success" : "neutral"}
                    />
                    <MetricCard
                      label="Needs attention"
                      value={String(advancedSummary.blocked)}
                      description="Enabled advanced features that still report blockers."
                      tone={advancedSummary.blocked > 0 ? "error" : "neutral"}
                    />
                    <MetricCard
                      label="Overrides"
                      value={String(advancedOverrides)}
                      description={`${advancedConfigurable} advanced features expose extra settings or dependencies.`}
                    />
                  </div>

                  <div className="commands-module-grid">
                    {backfillFeature !== null && backfillDetails !== null ? (
                      <section className="surface-subsection commands-module-panel">
                        <div className="commands-module-head">
                          <div className="card-copy commands-module-copy">
                            <p className="section-label">Backfill</p>
                            <div className="commands-module-title-row">
                              <h3>{backfillFeature.label}</h3>
                              <StatusBadge
                                tone={getFeatureStatusTone(backfillFeature)}
                              >
                                {formatFeatureStatusLabel(backfillFeature)}
                              </StatusBadge>
                            </div>
                            <p className="section-description">
                              Seed historical entry and exit metrics by choosing
                              the source channel and at least one schedule seed
                              date.
                            </p>
                          </div>

                          <span className="meta-pill subtle-pill">
                            {backfillFeature.override_state === "inherit"
                              ? "Using default"
                              : "Configured here"}
                          </span>
                        </div>

                        <KeyValueList
                          items={[
                            {
                              label: "Module state",
                              value: backfillFeature.effective_enabled
                                ? "On"
                                : "Off",
                            },
                            {
                              label: "Source channel",
                              value: formatGuildChannelValue(
                                backfillDetails.channelId,
                                channelOptions.channels,
                                "Not configured",
                              ),
                            },
                            {
                              label: "Schedule seed",
                              value:
                                formatBackfillScheduleValue(backfillDetails),
                            },
                            {
                              label: "Current signal",
                              value: summarizeBackfillSignal(backfillFeature),
                            },
                          ]}
                        />

                        <p className="meta-note commands-module-note">
                          {backfillDetails.channelId === ""
                            ? "Choose the source channel first, then set start day or initial date so the run knows where to begin."
                            : "Use start day or initial date to define how far back the recovery should begin for this server."}
                        </p>

                        {channelOptions.notice ? (
                          <div className="surface-subsection">
                            <p className="section-label">
                              Channel references unavailable
                            </p>
                            <p className="meta-note">
                              {channelOptions.notice.message}
                            </p>
                          </div>
                        ) : null}

                        <div className="inline-actions commands-module-actions">
                          <button
                            className="button-primary"
                            type="button"
                            disabled={!canEditBackfill(backfillFeature)}
                            onClick={() => openAdvancedDrawer(backfillFeature)}
                          >
                            Configure backfill
                          </button>
                          <button
                            className="button-secondary"
                            type="button"
                            disabled={featureMutation.saving}
                            onClick={() =>
                              void handleSetAdvancedFeatureEnabled(
                                backfillFeature.id,
                                !backfillFeature.effective_enabled,
                              )
                            }
                          >
                            {featureMutation.saving &&
                            pendingFeatureId === backfillFeature.id
                              ? "Saving..."
                              : backfillFeature.effective_enabled
                                ? "Disable backfill"
                                : "Enable backfill"}
                          </button>
                          {backfillFeature.override_state !== "inherit" ? (
                            <button
                              className="button-ghost"
                              type="button"
                              disabled={featureMutation.saving}
                              onClick={() =>
                                void handleUseAdvancedDefault(
                                  backfillFeature.id,
                                )
                              }
                            >
                              Use default
                            </button>
                          ) : null}
                        </div>
                      </section>
                    ) : null}

                    {userPruneFeature !== null && userPruneDetails !== null ? (
                      <section className="surface-subsection commands-module-panel">
                        <div className="commands-module-head">
                          <div className="card-copy commands-module-copy">
                            <p className="section-label">User prune</p>
                            <div className="commands-module-title-row">
                              <h3>{userPruneFeature.label}</h3>
                              <StatusBadge
                                tone={getFeatureStatusTone(userPruneFeature)}
                              >
                                {formatFeatureStatusLabel(userPruneFeature)}
                              </StatusBadge>
                            </div>
                            <p className="section-description">
                              Review the prune rule, pacing, and exempt roles
                              without exposing the whole advanced feature list
                              as the default editing surface.
                            </p>
                          </div>

                          <span className="meta-pill subtle-pill">
                            {formatUserPruneRunModeValue(
                              userPruneDetails.dryRun,
                            )}
                          </span>
                        </div>

                        <KeyValueList
                          items={[
                            {
                              label: "Module state",
                              value: userPruneFeature.effective_enabled
                                ? "On"
                                : "Off",
                            },
                            {
                              label: "Prune rule",
                              value: formatUserPruneRuleValue(
                                userPruneDetails.configEnabled,
                              ),
                            },
                            {
                              label: "Grace period",
                              value: `${userPruneDetails.graceDays} days`,
                            },
                            {
                              label: "Scan interval",
                              value: `${userPruneDetails.scanIntervalMins} minutes`,
                            },
                            {
                              label: "Exempt roles",
                              value:
                                formatUserPruneExemptRoleCountValue(
                                  userPruneDetails,
                                ),
                            },
                            {
                              label: "Current signal",
                              value: summarizeUserPruneSignal(userPruneFeature),
                            },
                          ]}
                        />

                        <p className="meta-note commands-module-note">
                          {formatUserPruneExemptRolesValue(
                            userPruneDetails,
                            roleOptions.roles,
                          )}
                        </p>

                        {roleOptions.notice ? (
                          <div className="surface-subsection">
                            <p className="section-label">
                              Role references unavailable
                            </p>
                            <p className="meta-note">
                              {roleOptions.notice.message}
                            </p>
                          </div>
                        ) : null}

                        <div className="inline-actions commands-module-actions">
                          <button
                            className="button-primary"
                            type="button"
                            disabled={!canEditUserPrune(userPruneFeature)}
                            onClick={() => openAdvancedDrawer(userPruneFeature)}
                          >
                            Configure user prune
                          </button>
                          <button
                            className="button-secondary"
                            type="button"
                            disabled={featureMutation.saving}
                            onClick={() =>
                              void handleSetAdvancedFeatureEnabled(
                                userPruneFeature.id,
                                !userPruneFeature.effective_enabled,
                              )
                            }
                          >
                            {featureMutation.saving &&
                            pendingFeatureId === userPruneFeature.id
                              ? "Saving..."
                              : userPruneFeature.effective_enabled
                                ? "Disable user prune"
                                : "Enable user prune"}
                          </button>
                          {userPruneFeature.override_state !== "inherit" ? (
                            <button
                              className="button-ghost"
                              type="button"
                              disabled={featureMutation.saving}
                              onClick={() =>
                                void handleUseAdvancedDefault(
                                  userPruneFeature.id,
                                )
                              }
                            >
                              Use default
                            </button>
                          ) : null}
                        </div>
                      </section>
                    ) : null}
                  </div>

                  {supportingAdvancedFeatures.length > 0 ? (
                    <div className="surface-subsection">
                      <div className="card-copy">
                        <p className="section-label">Supporting routines</p>
                        <h3>Compact runtime toggles</h3>
                        <p className="section-description">
                          Keep simpler cleanup and cache switches visible here
                          without promoting them to their own workspace cards.
                        </p>
                      </div>

                      <div className="table-wrap">
                        <table className="data-table feature-table">
                          <thead>
                            <tr>
                              <th scope="col">Feature</th>
                              <th scope="col">Status</th>
                              <th scope="col">Signal</th>
                              <th scope="col">Actions</th>
                            </tr>
                          </thead>
                          <tbody>
                            {supportingAdvancedFeatures.map((feature) => {
                              const isPending =
                                featureMutation.saving &&
                                pendingFeatureId === feature.id;

                              return (
                                <tr key={feature.id}>
                                  <td>
                                    <div className="feature-table-copy">
                                      <strong>{feature.label}</strong>
                                      <p>{feature.description}</p>
                                      <span className="meta-note">
                                        {formatOverrideLabel(
                                          feature.override_state,
                                        )}
                                      </span>
                                    </div>
                                  </td>
                                  <td>
                                    <div className="feature-status-cell">
                                      <StatusBadge
                                        tone={getFeatureStatusTone(feature)}
                                      >
                                        {formatFeatureStatusLabel(feature)}
                                      </StatusBadge>
                                      <span className="meta-note">
                                        {formatFeatureStatusSupport(feature)}
                                      </span>
                                    </div>
                                  </td>
                                  <td>
                                    <div className="feature-table-copy">
                                      <strong>
                                        {formatFeatureSignalTitle(feature)}
                                      </strong>
                                      <p>{formatFeatureSignal(feature)}</p>
                                      <span className="meta-note">
                                        {formatEffectiveSourceLabel(
                                          feature.effective_source,
                                        )}
                                      </span>
                                    </div>
                                  </td>
                                  <td>
                                    <div className="feature-row-actions">
                                      <button
                                        className="button-secondary"
                                        type="button"
                                        disabled={featureMutation.saving}
                                        aria-label={`${feature.effective_enabled ? "Disable" : "Enable"} ${feature.label}`}
                                        onClick={() =>
                                          void handleSetAdvancedFeatureEnabled(
                                            feature.id,
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
                                          disabled={featureMutation.saving}
                                          onClick={() =>
                                            void handleUseAdvancedDefault(
                                              feature.id,
                                            )
                                          }
                                        >
                                          Use default
                                        </button>
                                      ) : null}
                                    </div>
                                  </td>
                                </tr>
                              );
                            })}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  ) : null}

                  <div className="surface-subsection">
                    <p className="section-label">Guidance</p>
                    <ul className="feature-guidance-list">
                      <li>
                        Use this section for cleanup, cache, backfill, and prune
                        routines that should not occupy the main navigation.
                      </li>
                      <li>
                        Configure backfill and user prune through their drawers
                        so the main advanced page stays focused on the two
                        workflows that actually need extra inputs.
                      </li>
                      <li>
                        Use Diagnostics below only for raw destination IDs,
                        transport state, and OAuth troubleshooting.
                      </li>
                    </ul>

                    {advancedBlockingFeature ? (
                      <div className="workspace-footer">
                        <span className="meta-note">
                          Current blocker:{" "}
                          {advancedBlockingFeature.blockers?.[0]?.message ??
                            advancedSummary.signal}
                        </span>
                        <button
                          className="button-secondary"
                          type="button"
                          disabled={featureWorkspace.loading}
                          onClick={() => void handleRefreshAdvancedControls()}
                        >
                          Refresh advanced controls
                        </button>
                      </div>
                    ) : (
                      <div className="workspace-footer">
                        <span className="meta-note">
                          Advanced routines now live in Settings instead of a
                          standalone Maintenance surface.
                        </span>
                        <button
                          className="button-secondary"
                          type="button"
                          disabled={featureWorkspace.loading}
                          onClick={() => void handleRefreshAdvancedControls()}
                        >
                          Refresh advanced controls
                        </button>
                      </div>
                    )}
                  </div>
                </>
              )}
            </section>
          ) : null}
        </div>
      </SurfaceCard>

      {selectedAdvancedFeature !== null ? (
        <div
          className="drawer-backdrop"
          onClick={closeAdvancedDrawer}
          role="presentation"
        >
          <aside
            aria-label={getAdvancedDrawerLabel(selectedAdvancedFeature)}
            aria-modal="true"
            className="drawer-panel commands-drawer"
            onClick={(event) => event.stopPropagation()}
            role="dialog"
          >
            <div className="card-copy">
              <p className="section-label">Advanced</p>
              <div className="logging-drawer-title-row">
                <h2>{selectedAdvancedFeature.label}</h2>
                <StatusBadge
                  tone={getFeatureStatusTone(selectedAdvancedFeature)}
                >
                  {formatFeatureStatusLabel(selectedAdvancedFeature)}
                </StatusBadge>
              </div>
              <p className="section-description">
                {selectedAdvancedFeature.description}
              </p>
            </div>

            {featureMutation.notice ? (
              <AlertBanner notice={featureMutation.notice} />
            ) : null}

            {renderAdvancedDrawerBody({
              selectedFeature: selectedAdvancedFeature,
              pendingFeatureId,
              mutationSaving: featureMutation.saving,
              availableChannels: channelOptions.channels,
              channelOptions: backfillChannelPickerOptions,
              channelOptionsLoading: channelOptions.loading,
              channelOptionsNotice: channelOptions.notice,
              roleOptions: roleOptions.roles,
              roleOptionsLoading: roleOptions.loading,
              roleOptionsNotice: roleOptions.notice,
              backfillChannelDraft,
              backfillStartDayDraft,
              backfillInitialDateDraft,
              pruneConfigEnabledDraft,
              pruneGraceDaysDraft,
              pruneScanIntervalDraft,
              pruneInitialDelayDraft,
              pruneKicksPerSecondDraft,
              pruneMaxKicksPerRunDraft,
              pruneExemptRoleIDsDraft,
              pruneRunModeDraft,
              canSaveUserPrune,
              setBackfillChannelDraft,
              setBackfillStartDayDraft,
              setBackfillInitialDateDraft,
              setPruneConfigEnabledDraft,
              setPruneGraceDaysDraft,
              setPruneScanIntervalDraft,
              setPruneInitialDelayDraft,
              setPruneKicksPerSecondDraft,
              setPruneMaxKicksPerRunDraft,
              setPruneExemptRoleIDsDraft,
              setPruneRunModeDraft,
              refreshChannelOptions: channelOptions.refresh,
              refreshRoleOptions: roleOptions.refresh,
              closeDrawer: closeAdvancedDrawer,
              handleSaveBackfill,
              handleSaveUserPrune,
            })}
          </aside>
        </div>
      ) : null}

      <details
        className="details-panel surface-card diagnostics-panel settings-diagnostics"
        id="diagnostics"
        onToggle={(event) =>
          setDiagnosticsOpen((event.currentTarget as HTMLDetailsElement).open)
        }
        open={diagnosticsOpen}
      >
        <summary>Diagnostics</summary>

        {diagnosticsNoticeToRender || diagnosticsBusyLabel ? (
          <div className="diagnostics-alert">
            <AlertBanner
              notice={diagnosticsNoticeToRender}
              busyLabel={diagnosticsBusyLabel}
            />
          </div>
        ) : null}

        <div className="details-content diagnostics-content settings-diagnostics-content">
          <div className="card-copy settings-diagnostics-copy">
            <p className="section-label">Advanced</p>
            <h2>Diagnostics</h2>
            <p className="section-description">
              Inspect raw identifiers, OAuth scopes, and transport details only
              when you need to troubleshoot or complete a technical setup.
            </p>
          </div>

          <div className="settings-diagnostics-grid">
            <SurfaceCard className="settings-diagnostics-editor">
              <div className="card-copy">
                <p className="section-label">Partner Board destination</p>
                <h3>Advanced destination editor</h3>
                <p className="section-description">
                  This is the only place raw Discord identifiers and webhook
                  details are edited.
                </p>
              </div>

              <div className="workspace-form-grid">
                <label className="field-stack">
                  <span className="field-label">Posting method</span>
                  <select
                    value={deliveryForm.type}
                    onChange={(event) =>
                      setDeliveryForm((currentValue) => ({
                        ...currentValue,
                        type: event.target.value as DeliveryFormState["type"],
                      }))
                    }
                  >
                    <option value="channel_message">Channel message</option>
                    <option value="webhook_message">Webhook message</option>
                  </select>
                </label>

                <label className="field-stack">
                  <span className="field-label">Board message ID</span>
                  <input
                    value={deliveryForm.messageID}
                    onChange={(event) =>
                      setDeliveryForm((currentValue) => ({
                        ...currentValue,
                        messageID: event.target.value,
                      }))
                    }
                    placeholder="123456789012345678"
                  />
                </label>

                {deliveryForm.type === "webhook_message" ? (
                  <label className="field-stack">
                    <span className="field-label">Webhook URL</span>
                    <input
                      value={deliveryForm.webhookURL}
                      onChange={(event) =>
                        setDeliveryForm((currentValue) => ({
                          ...currentValue,
                          webhookURL: event.target.value,
                        }))
                      }
                      placeholder="https://discord.com/api/webhooks/..."
                    />
                  </label>
                ) : (
                  <label className="field-stack">
                    <span className="field-label">Channel ID</span>
                    <input
                      value={deliveryForm.channelID}
                      onChange={(event) =>
                        setDeliveryForm((currentValue) => ({
                          ...currentValue,
                          channelID: event.target.value,
                        }))
                      }
                      placeholder="123456789012345678"
                    />
                  </label>
                )}
              </div>

              <div className="workspace-footer">
                <button
                  className="button-primary"
                  type="button"
                  disabled={savingDelivery || boardSummaryLoading}
                  onClick={() => void handleSaveDelivery()}
                >
                  Save destination
                </button>
                <div className="workspace-toolbar-actions">
                  <span className="meta-pill subtle-pill">
                    {boardReadyLabel}
                  </span>
                  <span className="meta-pill subtle-pill">
                    {postingMethodLabel(deliveryForm.type)}
                  </span>
                </div>
              </div>
            </SurfaceCard>

            <SurfaceCard className="settings-diagnostics-state">
              <div className="card-copy">
                <p className="section-label">Technical state</p>
                <h3>Current session and board</h3>
                <p className="section-description">
                  Use these values when troubleshooting destination setup,
                  access, or session state.
                </p>
              </div>

              <KeyValueList
                className="workspace-status-list"
                items={[
                  {
                    label: "Selected server ID",
                    value: selectedGuildID || "No server selected",
                  },
                  {
                    label: "Granted OAuth scopes",
                    value:
                      session !== null && session.scopes.length > 0
                        ? session.scopes.join(", ")
                        : "Unavailable until sign-in",
                  },
                  {
                    label: "Current Partner Board method",
                    value: boardPostingMethodLabel,
                  },
                  {
                    label: "Destination completeness",
                    value: deliveryConfigured ? "Configured" : "Incomplete",
                  },
                  {
                    label: "Last board check",
                    value: formatTimestamp(lastLoadedAt, "Not checked yet"),
                  },
                  {
                    label: "Session guidance",
                    value: authSupportText,
                  },
                ]}
              />
            </SurfaceCard>
          </div>
        </div>
      </details>
    </section>
  );
}

interface RenderAdvancedDrawerBodyProps {
  selectedFeature: FeatureRecord;
  pendingFeatureId: string;
  mutationSaving: boolean;
  availableChannels: ReturnType<typeof useGuildChannelOptions>["channels"];
  channelOptions: Array<{ value: string; label: string; description?: string }>;
  channelOptionsLoading: boolean;
  channelOptionsNotice: Notice | null;
  roleOptions: ReturnType<typeof useGuildRoleOptions>["roles"];
  roleOptionsLoading: boolean;
  roleOptionsNotice: Notice | null;
  backfillChannelDraft: string;
  backfillStartDayDraft: string;
  backfillInitialDateDraft: string;
  pruneConfigEnabledDraft: string;
  pruneGraceDaysDraft: string;
  pruneScanIntervalDraft: string;
  pruneInitialDelayDraft: string;
  pruneKicksPerSecondDraft: string;
  pruneMaxKicksPerRunDraft: string;
  pruneExemptRoleIDsDraft: string[];
  pruneRunModeDraft: string;
  canSaveUserPrune: boolean;
  setBackfillChannelDraft: (value: string) => void;
  setBackfillStartDayDraft: (value: string) => void;
  setBackfillInitialDateDraft: (value: string) => void;
  setPruneConfigEnabledDraft: (value: string) => void;
  setPruneGraceDaysDraft: (value: string) => void;
  setPruneScanIntervalDraft: (value: string) => void;
  setPruneInitialDelayDraft: (value: string) => void;
  setPruneKicksPerSecondDraft: (value: string) => void;
  setPruneMaxKicksPerRunDraft: (value: string) => void;
  setPruneExemptRoleIDsDraft: (
    value: string[] | ((current: string[]) => string[]),
  ) => void;
  setPruneRunModeDraft: (value: string) => void;
  refreshChannelOptions: () => Promise<void>;
  refreshRoleOptions: () => Promise<void>;
  closeDrawer: () => void;
  handleSaveBackfill: () => Promise<void>;
  handleSaveUserPrune: () => Promise<void>;
}

function renderAdvancedDrawerBody({
  selectedFeature,
  pendingFeatureId,
  mutationSaving,
  availableChannels,
  channelOptions,
  channelOptionsLoading,
  channelOptionsNotice,
  roleOptions,
  roleOptionsLoading,
  roleOptionsNotice,
  backfillChannelDraft,
  backfillStartDayDraft,
  backfillInitialDateDraft,
  pruneConfigEnabledDraft,
  pruneGraceDaysDraft,
  pruneScanIntervalDraft,
  pruneInitialDelayDraft,
  pruneKicksPerSecondDraft,
  pruneMaxKicksPerRunDraft,
  pruneExemptRoleIDsDraft,
  pruneRunModeDraft,
  canSaveUserPrune,
  setBackfillChannelDraft,
  setBackfillStartDayDraft,
  setBackfillInitialDateDraft,
  setPruneConfigEnabledDraft,
  setPruneGraceDaysDraft,
  setPruneScanIntervalDraft,
  setPruneInitialDelayDraft,
  setPruneKicksPerSecondDraft,
  setPruneMaxKicksPerRunDraft,
  setPruneExemptRoleIDsDraft,
  setPruneRunModeDraft,
  refreshChannelOptions,
  refreshRoleOptions,
  closeDrawer,
  handleSaveBackfill,
  handleSaveUserPrune,
}: RenderAdvancedDrawerBodyProps) {
  if (selectedFeature.id === "backfill.enabled") {
    const details = getBackfillFeatureDetails(selectedFeature);

    return (
      <>
        <KeyValueList
          items={[
            {
              label: "Module state",
              value: selectedFeature.effective_enabled ? "On" : "Off",
            },
            {
              label: "Current channel",
              value: formatGuildChannelValue(
                details.channelId,
                availableChannels,
                "Not configured",
              ),
            },
            {
              label: "Schedule seed",
              value: formatBackfillScheduleValue(details),
            },
            {
              label: "Current signal",
              value: summarizeBackfillSignal(selectedFeature),
            },
          ]}
        />

        {channelOptionsNotice ? (
          <div className="surface-subsection">
            <p className="section-label">Channel references unavailable</p>
            <p className="meta-note">{channelOptionsNotice.message}</p>
            <div className="sidebar-actions">
              <button
                className="button-secondary"
                type="button"
                disabled={channelOptionsLoading}
                onClick={() => void refreshChannelOptions()}
              >
                Retry channel lookup
              </button>
            </div>
          </div>
        ) : null}

        <EntityPickerField
          label="Source channel"
          value={backfillChannelDraft}
          disabled={channelOptionsLoading}
          onChange={setBackfillChannelDraft}
          options={channelOptions}
          placeholder={
            channelOptionsLoading
              ? "Loading channels..."
              : channelOptions.length === 0
                ? "No channels available"
                : "No source channel"
          }
          note="Choose the channel that should seed the historical entry and exit scan."
        />

        <div className="field-grid roles-form-grid">
          <label className="field-stack">
            <span className="field-label">Start day</span>
            <input
              aria-label="Start day"
              type="date"
              value={backfillStartDayDraft}
              onChange={(event) => setBackfillStartDayDraft(event.target.value)}
            />
            <span className="meta-note">
              Use this when backfill should begin from a calendar day.
            </span>
          </label>

          <label className="field-stack">
            <span className="field-label">Initial date</span>
            <input
              aria-label="Initial date"
              type="date"
              value={backfillInitialDateDraft}
              onChange={(event) =>
                setBackfillInitialDateDraft(event.target.value)
              }
            />
            <span className="meta-note">
              Keep this as the explicit seed date when you need a fixed start.
            </span>
          </label>
        </div>

        <details className="details-panel">
          <summary>Advanced</summary>
          <div className="details-content">
            <label className="field-stack">
              <span className="field-label">Source channel ID fallback</span>
              <input
                aria-label="Source channel ID fallback"
                value={backfillChannelDraft}
                onChange={(event) =>
                  setBackfillChannelDraft(event.target.value)
                }
                placeholder="Discord channel ID"
              />
              <span className="meta-note">
                Use this only when the channel picker is unavailable and you
                need to paste the channel ID directly.
              </span>
            </label>
          </div>
        </details>

        <div className="surface-subsection">
          <p className="section-label">Backfill guidance</p>
          <ul className="feature-guidance-list">
            <li>
              Choose the source channel first so the backfill run knows where to
              scan.
            </li>
            <li>
              Set start day or initial date before enabling the module if you
              want readiness to turn green.
            </li>
          </ul>
        </div>

        <div className="drawer-actions">
          <button
            className="button-primary"
            type="button"
            disabled={mutationSaving}
            onClick={() => void handleSaveBackfill()}
          >
            {mutationSaving && pendingFeatureId === selectedFeature.id
              ? "Saving..."
              : "Save backfill"}
          </button>
          <button
            className="button-secondary"
            type="button"
            onClick={closeDrawer}
          >
            Cancel
          </button>
        </div>
      </>
    );
  }

  const details = getUserPruneFeatureDetails(selectedFeature);

  return (
    <>
      <KeyValueList
        items={[
          {
            label: "Module state",
            value: selectedFeature.effective_enabled ? "On" : "Off",
          },
          {
            label: "Prune rule",
            value: formatUserPruneRuleValue(details.configEnabled),
          },
          {
            label: "Run mode",
            value: formatUserPruneRunModeValue(details.dryRun),
          },
          {
            label: "Exempt roles",
            value: formatUserPruneExemptRoleCountValue(details),
          },
          {
            label: "Current signal",
            value: summarizeUserPruneSignal(selectedFeature),
          },
        ]}
      />

      <div className="field-grid roles-form-grid">
        <label className="field-stack">
          <span className="field-label">Prune rule</span>
          <select
            aria-label="Prune rule"
            value={pruneConfigEnabledDraft}
            onChange={(event) => setPruneConfigEnabledDraft(event.target.value)}
          >
            <option value="enabled">Enabled</option>
            <option value="disabled">Disabled</option>
          </select>
          <span className="meta-note">
            Keep the module on while pausing the prune rule itself when you need
            to preserve the rest of the configuration.
          </span>
        </label>

        <label className="field-stack">
          <span className="field-label">Grace period (days)</span>
          <input
            aria-label="Grace period (days)"
            inputMode="numeric"
            type="number"
            value={pruneGraceDaysDraft}
            onChange={(event) => setPruneGraceDaysDraft(event.target.value)}
          />
        </label>

        <label className="field-stack">
          <span className="field-label">Scan interval (minutes)</span>
          <input
            aria-label="Scan interval (minutes)"
            inputMode="numeric"
            type="number"
            value={pruneScanIntervalDraft}
            onChange={(event) => setPruneScanIntervalDraft(event.target.value)}
          />
        </label>

        <label className="field-stack">
          <span className="field-label">Run mode</span>
          <select
            aria-label="Run mode"
            value={pruneRunModeDraft}
            onChange={(event) => setPruneRunModeDraft(event.target.value)}
          >
            <option value="simulation">Simulation mode</option>
            <option value="live">Live run</option>
          </select>
        </label>
      </div>

      {roleOptionsNotice ? (
        <div className="surface-subsection">
          <p className="section-label">Role references unavailable</p>
          <p className="meta-note">{roleOptionsNotice.message}</p>
          <div className="sidebar-actions">
            <button
              className="button-secondary"
              type="button"
              disabled={roleOptionsLoading}
              onClick={() => void refreshRoleOptions()}
            >
              Retry role lookup
            </button>
          </div>
        </div>
      ) : roleOptionsLoading && roleOptions.length === 0 ? (
        <div className="surface-subsection">
          <p className="section-label">Loading roles</p>
          <p className="meta-note">
            Loading server roles before the exempt role list can be updated.
          </p>
        </div>
      ) : (
        <EntityMultiPickerField
          label="Exempt roles"
          disabled={roleOptionsLoading}
          selectedValues={pruneExemptRoleIDsDraft}
          onToggle={(roleId) =>
            setPruneExemptRoleIDsDraft((current) =>
              toggleExemptRole(current, roleId),
            )
          }
          options={roleOptions.map((role) => ({
            value: role.id,
            label: formatUserPruneRoleOptionLabel(role),
            description: role.is_default
              ? "Default role for every member."
              : role.managed
                ? "Managed by an integration."
                : "Members with this role are skipped during prune runs.",
          }))}
          note="Choose which roles should always stay out of the prune workflow."
        />
      )}

      <details className="details-panel">
        <summary>Advanced</summary>
        <div className="details-content">
          <div className="field-grid roles-form-grid">
            <label className="field-stack">
              <span className="field-label">Initial delay (seconds)</span>
              <input
                aria-label="Initial delay (seconds)"
                inputMode="numeric"
                type="number"
                value={pruneInitialDelayDraft}
                onChange={(event) =>
                  setPruneInitialDelayDraft(event.target.value)
                }
              />
            </label>

            <label className="field-stack">
              <span className="field-label">Kicks per second</span>
              <input
                aria-label="Kicks per second"
                inputMode="numeric"
                type="number"
                value={pruneKicksPerSecondDraft}
                onChange={(event) =>
                  setPruneKicksPerSecondDraft(event.target.value)
                }
              />
            </label>

            <label className="field-stack">
              <span className="field-label">Max kicks per run</span>
              <input
                aria-label="Max kicks per run"
                inputMode="numeric"
                type="number"
                value={pruneMaxKicksPerRunDraft}
                onChange={(event) =>
                  setPruneMaxKicksPerRunDraft(event.target.value)
                }
              />
            </label>
          </div>
        </div>
      </details>

      {!canSaveUserPrune ? (
        <div className="surface-subsection">
          <p className="section-label">Values required</p>
          <p className="meta-note">
            Fill every numeric prune field with a whole number before saving the
            prune configuration.
          </p>
        </div>
      ) : null}

      <div className="surface-subsection">
        <p className="section-label">Prune guidance</p>
        <ul className="feature-guidance-list">
          <li>
            Use simulation mode first when you want to inspect the prune plan
            before allowing live actions.
          </li>
          <li>
            Keep staff and protected roles in the exempt list so routine scans
            skip them automatically.
          </li>
        </ul>
      </div>

      <div className="drawer-actions">
        <button
          className="button-primary"
          type="button"
          disabled={mutationSaving || !canSaveUserPrune}
          onClick={() => void handleSaveUserPrune()}
        >
          {mutationSaving && pendingFeatureId === selectedFeature.id
            ? "Saving..."
            : "Save user prune"}
        </button>
        <button
          className="button-secondary"
          type="button"
          onClick={closeDrawer}
        >
          Cancel
        </button>
      </div>
    </>
  );
}

function getAdvancedDrawerLabel(feature: FeatureRecord) {
  switch (feature.id) {
    case "backfill.enabled":
      return "Configure entry and exit backfill";
    case "user_prune":
      return "Configure user prune";
    default:
      return `Configure ${feature.label}`;
  }
}

function resolveSettingsSection(hash: string): SettingsSection {
  switch (hash) {
    case "#connection":
      return "connection";
    case "#permissions":
      return "permissions";
    case "#advanced":
    case "#diagnostics":
      return "advanced";
    default:
      return "server";
  }
}
