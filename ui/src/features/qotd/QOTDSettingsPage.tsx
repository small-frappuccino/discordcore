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
import { useQOTD } from "./QOTDContext";

interface SettingsDraft {
  enabled: boolean;
  forum_channel_id: string;
  question_tag_id: string;
  reply_tag_id: string;
  staff_role_ids: string[];
}

export function QOTDSettingsPage() {
  const { canEditSelectedGuild } = useDashboardSession();
  const { forumTags, refreshForumTags, saveSettings, settings, summary } = useQOTD();
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
    <div className="control-panel-grid">
      <SurfaceCard className="control-panel-card">
        <div className="card-copy">
          <p className="section-label">Forum target</p>
          <h2>Official QOTD forum</h2>
          <p className="section-description">
            Pick the Discord forum channel that will hold the official locked QOTD posts and member reply threads.
          </p>
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
              Requires a forum channel plus distinct Question and Reply tags before publish-now is allowed.
            </span>
          </span>
        </label>

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
          note="Use the same forum for official question posts and member reply posts."
        />
      </SurfaceCard>

      <SurfaceCard className="control-panel-card">
        <div className="card-copy">
          <p className="section-label">Forum tags</p>
          <h2>Question and reply routing tags</h2>
          <p className="section-description">
            Map the existing forum tags that distinguish official QOTD posts from member-created reply threads.
          </p>
        </div>

        {!tagLookupAvailable ? (
          <p className="meta-note">Select a forum channel first to load its tags.</p>
        ) : null}

        <div className="inline-actions">
          <button
            className="button-secondary"
            type="button"
            disabled={controlsDisabled || !tagLookupAvailable}
            onClick={() => void refreshForumTags(draft.forum_channel_id)}
          >
            Refresh tags
          </button>
        </div>

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
          note="Apply this tag to the official locked daily QOTD posts."
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
          note="Apply this tag to member reply threads created through the Answer button."
        />
      </SurfaceCard>

      <SurfaceCard className="control-panel-card control-panel-card-wide">
        <div className="card-copy">
          <p className="section-label">Staff handling</p>
          <h2>Moderation roles for locked official posts</h2>
          <p className="section-description">
            These roles are reserved for direct moderation or bot-side handling in the official question posts.
          </p>
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
          note="This list is stored now for backend enforcement and moderation flows; it does not change Discord permissions directly."
        />

        <div className="surface-subsection">
          <p className="section-label">Current slot</p>
          <p className="meta-note">
            {summary?.published_for_current_slot
              ? "The due UTC slot already has an official QOTD post."
              : "The due UTC slot has not been published yet."}
          </p>
          <div className="inline-actions">
            <button
              className="button-primary"
              type="button"
              disabled={controlsDisabled || !hasUnsavedChanges}
              onClick={() => void handleSave()}
            >
              {saving ? "Saving..." : "Save QOTD settings"}
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
