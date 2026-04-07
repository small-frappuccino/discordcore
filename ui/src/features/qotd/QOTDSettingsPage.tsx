import { useEffect, useRef, useState } from "react";
import type { QOTDConfig } from "../../api/control";
import {
  EntityMultiPickerField,
  EntityPickerField,
  UnsavedChangesBar,
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
  const { busyLabel, forumTags, refreshForumTags, saveSettings, settings } = useQOTD();
  const channelOptions = useGuildChannelOptions();
  const roleOptions = useGuildRoleOptions();
  const savedDraftRef = useRef<SettingsDraft>(createSettingsDraft(settings));
  const [draft, setDraft] = useState<SettingsDraft>(() => savedDraftRef.current);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    const nextSavedDraft = createSettingsDraft(settings);
    const previousSavedDraft = savedDraftRef.current;
    savedDraftRef.current = nextSavedDraft;
    setDraft((currentDraft) =>
      settingsDraftChanged(previousSavedDraft, currentDraft)
        ? currentDraft
        : nextSavedDraft,
    );
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
    description: tag.moderated ? "Moderated forum tag." : "Standard forum tag.",
  }));
  const hasUnsavedChanges = settingsDraftChanged(savedDraftRef.current, draft);
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

  function handleReset() {
    const nextDraft = savedDraftRef.current;
    setDraft(nextDraft);
    void refreshForumTags(nextDraft.forum_channel_id);
  }

  return (
    <div className="workspace-view qotd-workspace">
      <section className="qotd-flat-section">
        <div className="qotd-section-header">
          <div className="card-copy">
            <p className="section-label">Settings</p>
            <h2>Workflow settings</h2>
            <p className="section-description">
              Choose the forum and tags used by the daily publish flow.
            </p>
          </div>
        </div>

        {channelOptions.notice ? (
          <div className="qotd-flat-inline-message">
            <p className="meta-note">{channelOptions.notice.message}</p>
            <div className="inline-actions">
              <button
                className="button-secondary"
                type="button"
                disabled={channelOptions.loading}
                onClick={() => void channelOptions.refresh()}
              >
                Retry channel lookup
              </button>
            </div>
          </div>
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
              Turns on daily publishing for the selected forum route.
            </span>
          </span>
        </label>

        <div className="qotd-settings-fields">
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
            note="Use one forum for the official post and member reply threads."
          />

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
            note="Applied to the official daily post."
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

        {!tagLookupAvailable ? (
          <p className="meta-note">Choose a forum channel before selecting tags.</p>
        ) : refreshingTags ? (
          <p className="meta-note">Loading forum tags for the selected forum.</p>
        ) : null}
      </section>

      <section className="qotd-flat-section">
        <div className="qotd-section-header">
          <div className="card-copy">
            <p className="section-label">Staff</p>
            <h2>Staff roles</h2>
            <p className="section-description">
              Stored for moderation and official post handling flows.
            </p>
          </div>
          <div className="inline-actions">
            <span className="meta-pill subtle-pill">{draft.staff_role_ids.length} roles</span>
          </div>
        </div>

        {roleOptions.notice ? (
          <div className="qotd-flat-inline-message">
            <p className="meta-note">{roleOptions.notice.message}</p>
            <div className="inline-actions">
              <button
                className="button-secondary"
                type="button"
                disabled={roleOptions.loading}
                onClick={() => void roleOptions.refresh()}
              >
                Retry role lookup
              </button>
            </div>
          </div>
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
          note="Stored for backend moderation flows."
        />

      </section>

      <UnsavedChangesBar
        hasUnsavedChanges={hasUnsavedChanges}
        saveLabel={saving ? "Saving..." : "Save changes"}
        saving={saving}
        disabled={controlsDisabled}
        onReset={handleReset}
        onSave={handleSave}
      />
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
