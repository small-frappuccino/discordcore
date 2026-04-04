import { useEffect, useState } from "react";
import type { QOTDConfig } from "../../api/control";
import {
  EntityMultiPickerField,
  EntityPickerField,
  LookupNotice,
  SurfaceCard,
} from "../../components/ui";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useGuildChannelOptions } from "../features/useGuildChannelOptions";
import { useGuildRoleOptions } from "../features/useGuildRoleOptions";
import { formatRoleOptionLabel } from "../features/roles";
import { QOTD_BUSY_LABELS, useQOTD } from "./QOTDContext";

interface SettingsDraft {
  enabled: boolean;
  forum_channel_id: string;
  question_tag_id: string;
  reply_tag_id: string;
  staff_role_ids: string[];
}

export function QOTDSettingsPage() {
  const { canEditSelectedGuild } = useDashboardSession();
  const { busyLabel, forumTags, refreshForumTags, saveSettings, settings, summary } = useQOTD();
  const channelOptions = useGuildChannelOptions();
  const roleOptions = useGuildRoleOptions();
  const [draft, setDraft] = useState<SettingsDraft>(() => createSettingsDraft(settings));
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setDraft(createSettingsDraft(settings));
  }, [settings]);

  const forumChannelOptions = channelOptions.channels
    .filter((channel) => channel.kind === "forum")
    .map((channel) => ({
      value: channel.id,
      label: channel.display_name,
      description: "Forum channel available for official QOTD and reply posts.",
    }));
  const rolePickerOptions = roleOptions.roles.map((role) => ({
    value: role.id,
    label: formatRoleOptionLabel(role),
    description: role.is_default
      ? "Default role for every member."
      : role.managed
        ? "Managed by an integration."
        : "Available for staff-only post handling.",
  }));
  const tagOptions = forumTags.map((tag) => ({
    value: tag.id,
    label: tag.name,
    description: tag.moderated
      ? "Moderated forum tag."
      : "Standard forum tag.",
  }));
  const hasUnsavedChanges = settingsDraftChanged(settings, draft);
  const controlsDisabled = !canEditSelectedGuild || saving;
  const tagLookupAvailable = draft.forum_channel_id.trim() !== "";
  const refreshingTags = busyLabel === QOTD_BUSY_LABELS.refreshForumTags;

  async function handleSave() {
    if (controlsDisabled) {
      return;
    }

    setSaving(true);
    try {
      await saveSettings(draft);
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="qotd-settings-grid">
      <SurfaceCard className="qotd-panel-card qotd-settings-card-wide">
        <div className="qotd-card-head">
          <div className="card-copy">
            <p className="section-label">Setup</p>
            <h3>Workflow routing</h3>
            <p className="section-description">
              Choose the forum used by the official daily post and its linked answer threads.
            </p>
          </div>
          <div className="qotd-card-meta">
            <span className={`qotd-status-pill ${draft.enabled ? "qotd-status-success" : "qotd-status-neutral"}`}>
              {draft.enabled ? "Enabled" : "Disabled"}
            </span>
            <span className="meta-pill subtle-pill">
              {draft.forum_channel_id ? "Forum selected" : "Forum missing"}
            </span>
          </div>
        </div>

        {channelOptions.notice ? (
          <LookupNotice
            title="Forum channels unavailable"
            message={channelOptions.notice.message}
            retryLabel="Retry channel lookup"
            retryDisabled={channelOptions.loading}
            onRetry={channelOptions.refresh}
          />
        ) : null}

        <label className="entity-option-card">
          <input
            checked={draft.enabled}
            disabled={controlsDisabled}
            type="checkbox"
            onChange={(event) =>
              setDraft((current) => ({
                ...current,
                enabled: event.target.checked,
              }))
            }
          />
          <span className="entity-option-copy">
            <strong>Enable QOTD workflow</strong>
            <span className="meta-note">
              Keep this on when the forum and tag routing are ready for the daily publish flow.
            </span>
          </span>
        </label>

        <div className="qotd-form-grid">
          <EntityPickerField
            label="Forum channel"
            value={draft.forum_channel_id}
            onChange={(value) => {
              setDraft((current) => ({
                ...current,
                forum_channel_id: value,
                question_tag_id: "",
                reply_tag_id: "",
              }));
              void refreshForumTags(value);
            }}
            options={forumChannelOptions}
            placeholder="Select a forum channel"
            disabled={controlsDisabled || channelOptions.loading}
            note="Use one forum for both the official post and member-created answer threads."
          />

          <div className="qotd-support-card">
            <p className="section-label">Current slot</p>
            <strong>{summary?.current_publish_date_utc?.slice(0, 10) ?? "No active slot"}</strong>
            <p className="meta-note">
              {summary?.published_for_current_slot
                ? "This slot already has an official QOTD post."
                : "This slot is still waiting for an official post."}
            </p>
          </div>
        </div>
      </SurfaceCard>

      <SurfaceCard className="qotd-panel-card">
        <div className="qotd-card-head">
          <div className="card-copy">
            <p className="section-label">Tags</p>
            <h3>Forum routing tags</h3>
            <p className="section-description">
              Map the official question tag and the member reply tag on the selected forum.
            </p>
          </div>
          <div className="inline-actions">
            <button
              className="button-secondary"
              type="button"
              disabled={controlsDisabled || !tagLookupAvailable}
              onClick={() => void refreshForumTags(draft.forum_channel_id)}
            >
              {refreshingTags ? "Refreshing..." : "Refresh tags"}
            </button>
          </div>
        </div>

        {!tagLookupAvailable ? (
          <div className="qotd-inline-note">
            <p className="meta-note">Select a forum channel first to load its tags.</p>
          </div>
        ) : null}

        <div className="qotd-settings-stack">
          <EntityPickerField
            label="Question tag"
            value={draft.question_tag_id}
            onChange={(value) =>
              setDraft((current) => ({
                ...current,
                question_tag_id: value,
              }))
            }
            options={tagOptions}
            placeholder="Select the Question tag"
            disabled={controlsDisabled || !tagLookupAvailable}
            note="Applied to the locked daily QOTD post."
          />

          <EntityPickerField
            label="Reply tag"
            value={draft.reply_tag_id}
            onChange={(value) =>
              setDraft((current) => ({
                ...current,
                reply_tag_id: value,
              }))
            }
            options={tagOptions}
            placeholder="Select the Reply tag"
            disabled={controlsDisabled || !tagLookupAvailable}
            note="Applied to member-created answer threads."
          />
        </div>
      </SurfaceCard>

      <SurfaceCard className="qotd-panel-card">
        <div className="qotd-card-head">
          <div className="card-copy">
            <p className="section-label">Staff</p>
            <h3>Moderation coverage</h3>
            <p className="section-description">
              Roles stored here are reserved for backend enforcement and official post handling.
            </p>
          </div>
          <div className="qotd-card-meta">
            <span className="meta-pill subtle-pill">{draft.staff_role_ids.length} roles</span>
          </div>
        </div>

        {roleOptions.notice ? (
          <LookupNotice
            title="Role references unavailable"
            message={roleOptions.notice.message}
            retryLabel="Retry role lookup"
            retryDisabled={roleOptions.loading}
            onRetry={roleOptions.refresh}
          />
        ) : null}

        <EntityMultiPickerField
          label="Staff roles"
          options={rolePickerOptions}
          selectedValues={draft.staff_role_ids}
          disabled={controlsDisabled || roleOptions.loading}
          onToggle={(roleId) =>
            setDraft((current) => ({
              ...current,
              staff_role_ids: toggleStringValue(current.staff_role_ids, roleId),
            }))
          }
          note="This list is stored for backend moderation flows; it does not change Discord permissions directly."
        />

        <div className="qotd-settings-footer">
          <div className="qotd-support-card">
            <p className="section-label">Queue</p>
            <strong>{summary?.counts.ready ?? 0} ready</strong>
            <p className="meta-note">
              {summary?.counts.ready
                ? "The bank already has ready questions for future slots."
                : "Add at least one ready question to cover the next slot."}
            </p>
          </div>

          <div className="inline-actions">
            <button
              className="button-primary"
              type="button"
              disabled={controlsDisabled || !hasUnsavedChanges}
              onClick={() => void handleSave()}
            >
              {saving ? "Saving..." : "Save changes"}
            </button>
          </div>
        </div>
      </SurfaceCard>
    </div>
  );
}

function createSettingsDraft(settings: Partial<SettingsDraft> | QOTDConfig): SettingsDraft {
  return {
    enabled: Boolean(settings.enabled),
    forum_channel_id: String(settings.forum_channel_id ?? ""),
    question_tag_id: String(settings.question_tag_id ?? ""),
    reply_tag_id: String(settings.reply_tag_id ?? ""),
    staff_role_ids: normalizeStrings(
      Array.isArray(settings.staff_role_ids) ? settings.staff_role_ids : [],
    ),
  };
}

function settingsDraftChanged(previous: Partial<SettingsDraft> | QOTDConfig, next: SettingsDraft) {
  const normalizedPrevious = createSettingsDraft(previous);
  return (
    normalizedPrevious.enabled !== next.enabled ||
    normalizedPrevious.forum_channel_id !== next.forum_channel_id ||
    normalizedPrevious.question_tag_id !== next.question_tag_id ||
    normalizedPrevious.reply_tag_id !== next.reply_tag_id ||
    normalizedPrevious.staff_role_ids.join("|") !== next.staff_role_ids.join("|")
  );
}

function toggleStringValue(values: string[], nextValue: string) {
  const normalized = nextValue.trim();
  if (normalized === "") {
    return normalizeStrings(values);
  }

  const set = new Set(normalizeStrings(values));
  if (set.has(normalized)) {
    set.delete(normalized);
  } else {
    set.add(normalized);
  }
  return Array.from(set).sort();
}

function normalizeStrings(values: unknown[]) {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const value of values) {
    const normalized = String(value ?? "").trim();
    if (normalized === "" || seen.has(normalized)) {
      continue;
    }
    seen.add(normalized);
    out.push(normalized);
  }
  return out.sort();
}
