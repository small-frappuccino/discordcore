import { NavLink, Outlet } from "react-router-dom";
import { buildQOTDTabs } from "../../app/routes";
import { formatTimestamp } from "../../app/utils";
import {
  EmptyState,
  FeatureWorkspaceLayout,
  MetricCard,
  PageHeader,
  StatusBadge,
  SurfaceCard,
} from "../../components/ui";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { QOTD_BUSY_LABELS, useQOTD } from "./QOTDContext";

const emptyQueueCounts = {
  total: 0,
  draft: 0,
  ready: 0,
  reserved: 0,
  used: 0,
  disabled: 0,
};

export function QOTDLayout() {
  const {
    authState,
    beginLogin,
    canEditSelectedGuild,
    selectedGuild,
    selectedGuildID,
  } = useDashboardSession();
  const {
    busyLabel,
    loading,
    notice,
    refreshWorkspace,
    publishNow,
    settings,
    summary,
    workspaceState,
  } = useQOTD();
  const tabs = buildQOTDTabs(selectedGuildID);
  const queueCounts = summary?.counts ?? emptyQueueCounts;
  const readyCount = queueCounts.ready;
  const status = buildShellStatus({
    enabled: settings.enabled ?? false,
    forumChannelId: settings.forum_channel_id ?? "",
    questionTagId: settings.question_tag_id ?? "",
    replyTagId: settings.reply_tag_id ?? "",
    publishedForCurrentSlot: summary?.published_for_current_slot ?? false,
    readyCount,
  });
  const currentSlotLabel = summary?.current_publish_date_utc?.slice(0, 10) ?? "No active slot";
  const currentPostLabel = summary?.current_post?.published_at
    ? `Published ${formatTimestamp(Date.parse(summary.current_post.published_at))}`
    : summary?.published_for_current_slot
      ? "Current slot has an official post."
      : "Current slot still open.";
  const previousPostLabel = summary?.previous_post?.publish_date_utc
    ? `Previous slot ${summary.previous_post.publish_date_utc.slice(0, 10)}`
    : "No previous slot active.";
  const latestPost = summary?.current_post ?? summary?.previous_post ?? null;
  const latestPostLabel = latestPost?.published_at
    ? `Published ${formatTimestamp(Date.parse(latestPost.published_at))}`
    : latestPost?.publish_date_utc
      ? `Slot ${latestPost.publish_date_utc.slice(0, 10)}`
      : "No official post history yet.";
  const selectedServerLabel = selectedGuild?.name ?? "No server selected";
  const isRefreshing = loading && busyLabel === QOTD_BUSY_LABELS.refreshWorkspace;
  const isPublishing = loading && busyLabel === QOTD_BUSY_LABELS.publishNow;
  const publishDisabled =
    authState !== "signed_in" ||
    !canEditSelectedGuild ||
    !(settings.enabled ?? false) ||
    loading;

  function renderBody() {
    if (workspaceState === "checking" || workspaceState === "loading") {
      return (
        <SurfaceCard className="empty-state-card">
          <p className="section-label">Workspace</p>
          <h2>Loading QOTD</h2>
          <p>Preparing workflow settings, queue health, and the current publish slot.</p>
        </SurfaceCard>
      );
    }

    if (workspaceState === "auth_required") {
      return (
        <EmptyState
          title="Sign in required"
          description="Sign in with Discord before managing the QOTD workflow."
          action={
            <button
              className="button-primary"
              type="button"
              onClick={() => void beginLogin()}
            >
              Sign in with Discord
            </button>
          }
        />
      );
    }

    if (workspaceState === "server_required") {
      return (
        <EmptyState
          title="Select a server"
          description="Choose a server from the top bar before editing workflow settings or queue order."
        />
      );
    }

    if (workspaceState === "unavailable") {
      return (
        <EmptyState
          title="QOTD unavailable"
          description="The workspace could not be loaded for this server. Try refreshing the page data."
          action={
            <button
              className="button-secondary"
              type="button"
              disabled={loading}
              onClick={() => void refreshWorkspace()}
            >
              Refresh QOTD
            </button>
          }
        />
      );
    }

    return (
      <FeatureWorkspaceLayout
        notice={notice}
        surfaceClassName="qotd-page-surface"
        summary={
          <section aria-label="QOTD summary" className="qotd-summary-grid">
            <MetricCard
              className="qotd-summary-card"
              label="Workflow"
              value={status.label}
              description={status.description}
              tone={status.tone}
            />
            <MetricCard
              className="qotd-summary-card"
              label="Publish slot"
              value={currentSlotLabel}
              description={currentPostLabel}
              tone={summary?.published_for_current_slot ? "success" : "info"}
            />
            <MetricCard
              className="qotd-summary-card"
              label="Ready queue"
              value={String(readyCount)}
              description={`${queueCounts.draft} draft · ${queueCounts.reserved} reserved`}
              tone={readyCount > 0 ? "success" : "neutral"}
            />
            <MetricCard
              className="qotd-summary-card"
              label="Latest post"
              value={summary?.current_post ? "Current slot" : summary?.previous_post ? "Previous slot" : "None"}
              description={latestPostLabel}
              tone={summary?.current_post ? "success" : "neutral"}
            />
          </section>
        }
        workspaceEyebrow="Workflow"
        workspaceTitle="Daily publishing flow"
        workspaceDescription="Keep routing, queue health, and publish coverage aligned with the current slot."
        workspaceMeta={
          <>
            <span className="meta-pill subtle-pill">{currentSlotLabel}</span>
            <span className="meta-pill subtle-pill">{readyCount} ready</span>
          </>
        }
        workspaceClassName="qotd-workspace-panel"
        workspaceContent={
          <div className="qotd-workspace-stack">
            <nav className="subnav workspace-tabs qotd-tabs" aria-label="QOTD sections">
              {tabs.map((tab) => (
                <NavLink
                  key={tab.path}
                  className={({ isActive }) =>
                    `subnav-link${isActive ? " is-active" : ""}`
                  }
                  to={tab.path}
                >
                  {tab.label}
                </NavLink>
              ))}
            </nav>

            <div className="qotd-tab-panel">
              <Outlet />
            </div>
          </div>
        }
        aside={
          <aside className="page-aside qotd-sidebar">
            <SurfaceCard className="qotd-side-card">
              <div className="card-copy">
                <p className="section-label">Setup</p>
                <h3>Setup readiness</h3>
                <p className="section-description">
                  The workflow reads more clearly when forum routing and queue coverage are complete.
                </p>
              </div>

              <ul className="qotd-readiness-list">
                {buildReadinessItems(settings, readyCount).map((item) => (
                  <li
                    key={item.label}
                    className={`qotd-readiness-item${item.complete ? " is-complete" : ""}`}
                  >
                    <span className="qotd-readiness-copy">
                      <span className="qotd-readiness-dot" aria-hidden="true" />
                      <strong>{item.label}</strong>
                    </span>
                    <span>{item.value}</span>
                  </li>
                ))}
              </ul>
            </SurfaceCard>

            <SurfaceCard className="qotd-side-card">
              <div className="card-copy">
                <p className="section-label">Queue</p>
                <h3>Queue mix</h3>
                <p className="section-description">
                  Draft, ready, and reserved counts show whether the next slot is covered.
                </p>
              </div>

              <dl className="qotd-stat-list">
                {buildQueueStats(queueCounts).map((item) => (
                  <div className="qotd-stat-row" key={item.label}>
                    <dt>{item.label}</dt>
                    <dd>{item.value}</dd>
                  </div>
                ))}
              </dl>

              <div className="qotd-side-note">
                <p className="section-label">Latest official post</p>
                <strong>
                  {summary?.current_post
                    ? "Current slot live"
                    : summary?.previous_post
                      ? "Previous slot available"
                      : "No official post yet"}
                </strong>
                <p className="meta-note">
                  {summary?.current_post ? currentPostLabel : previousPostLabel}
                </p>
                {latestPost?.question_text_snapshot ? (
                  <p className="qotd-post-preview">{latestPost.question_text_snapshot}</p>
                ) : null}
              </div>
            </SurfaceCard>
          </aside>
        }
      />
    );
  }

  return (
    <section className="page-shell qotd-page">
      <PageHeader
        eyebrow="Engagement"
        title="QOTD"
        description="Run the forum-based Question of the Day workflow, keep the bank healthy, and publish manual posts when needed."
        status={<StatusBadge tone={status.tone}>{status.label}</StatusBadge>}
        meta={
          <>
            <span className="meta-pill subtle-pill">{selectedServerLabel}</span>
            <span className="meta-pill subtle-pill">{currentSlotLabel}</span>
          </>
        }
        actions={
          <div className="inline-actions">
            <button
              className="button-secondary"
              type="button"
              disabled={loading}
              onClick={() => void refreshWorkspace()}
            >
              {isRefreshing ? "Refreshing..." : "Refresh"}
            </button>
            <button
              className="button-primary"
              type="button"
              disabled={publishDisabled}
              onClick={() => void publishNow()}
            >
              {isPublishing ? "Publishing..." : "Publish manual QOTD"}
            </button>
          </div>
        }
      />

      {renderBody()}
    </section>
  );
}

function buildShellStatus({
  enabled,
  forumChannelId,
  questionTagId,
  replyTagId,
  publishedForCurrentSlot,
  readyCount,
}: {
  enabled: boolean;
  forumChannelId: string;
  questionTagId: string;
  replyTagId: string;
  publishedForCurrentSlot: boolean;
  readyCount: number;
}) {
  if (!enabled) {
    return {
      tone: "info" as const,
      label: "Disabled",
      description: "Enable the workflow before the daily publish flow can run.",
    };
  }
  if (
    forumChannelId.trim() === "" ||
    questionTagId.trim() === "" ||
    replyTagId.trim() === ""
  ) {
    return {
      tone: "info" as const,
      label: "Needs setup",
      description: "Forum routing is incomplete for official posts and replies.",
    };
  }
  if (publishedForCurrentSlot) {
    return {
      tone: "success" as const,
      label: "Current slot published",
      description: "The due UTC slot already has an official Discord post.",
    };
  }
  if (readyCount <= 0) {
    return {
      tone: "neutral" as const,
      label: "Queue light",
      description: "Add a ready question so the next slot has coverage.",
    };
  }
  return {
    tone: "info" as const,
    label: "Queue ready",
    description: "The current slot is open and the bank already has ready questions.",
  };
}

function buildReadinessItems(
  settings: {
    forum_channel_id?: string;
    question_tag_id?: string;
    reply_tag_id?: string;
  },
  readyCount: number,
) {
  return [
    {
      label: "Forum channel",
      value: settings.forum_channel_id?.trim() ? "Connected" : "Missing",
      complete: Boolean(settings.forum_channel_id?.trim()),
    },
    {
      label: "Question tag",
      value: settings.question_tag_id?.trim() ? "Mapped" : "Missing",
      complete: Boolean(settings.question_tag_id?.trim()),
    },
    {
      label: "Reply tag",
      value: settings.reply_tag_id?.trim() ? "Mapped" : "Missing",
      complete: Boolean(settings.reply_tag_id?.trim()),
    },
    {
      label: "Ready questions",
      value: readyCount > 0 ? `${readyCount} queued` : "None ready",
      complete: readyCount > 0,
    },
  ];
}

function buildQueueStats(counts: typeof emptyQueueCounts) {
  return [
    { label: "Draft", value: counts.draft },
    { label: "Ready", value: counts.ready },
    { label: "Reserved", value: counts.reserved },
    { label: "Used", value: counts.used },
    { label: "Disabled", value: counts.disabled },
  ];
}
