import { useEffect, useId, useRef, useState } from "react";
import type { QOTDConfig } from "../../api/control";
import {
  GroupedSettingsCopy,
  GroupedSettingsGroup,
  GroupedSettingsHeading,
  GroupedSettingsInlineMessage,
  GroupedSettingsItem,
  GroupedSettingsMainRow,
  GroupedSettingsSection,
  GroupedSettingsStack,
  GroupedSettingsSubrow,
  GroupedSettingsSwitch,
  SettingsSelectField,
  UnsavedChangesBar,
} from "../../components/ui";
import { useDashboardSession } from "../../context/DashboardSessionContext";
import { useGuildChannelOptions } from "../features/useGuildChannelOptions";
import { QOTD_BUSY_LABELS, useQOTD } from "./QOTDContext";

interface SettingsDraft {
  enabled: boolean;
  forum_channel_id: string;
  question_tag_id: string;
  reply_tag_id: string;
}

export function QOTDSettingsPage() {
  const { canEditSelectedGuild } = useDashboardSession();
  const { busyLabel, forumTags, refreshForumTags, saveSettings, settings } = useQOTD();
  const channelOptions = useGuildChannelOptions();
  const workflowHeadingId = useId();
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
  const tagOptions = forumTags.map((tag) => ({
    value: tag.id,
    label: tag.name,
    description: tag.moderated ? "Moderated forum tag." : "Standard forum tag.",
  }));
  const hasUnsavedChanges = settingsDraftChanged(savedDraftRef.current, draft);
  const controlsDisabled = !canEditSelectedGuild || saving;
  const tagLookupAvailable = draft.forum_channel_id.trim() !== "";
  const refreshingTags = busyLabel === QOTD_BUSY_LABELS.refreshForumTags;
  const forumChannelPlaceholder = channelOptions.loading
    ? "Loading forum channels..."
    : forumChannelOptions.length === 0
      ? "No forum channels available"
      : "Select a forum channel";
  const questionTagPlaceholder = !tagLookupAvailable
    ? "Choose a forum channel first"
    : refreshingTags
      ? "Loading forum tags..."
      : tagOptions.length === 0
        ? "No forum tags available"
        : "Select the Question tag";
  const replyTagPlaceholder = !tagLookupAvailable
    ? "Choose a forum channel first"
    : refreshingTags
      ? "Loading forum tags..."
      : tagOptions.length === 0
        ? "No forum tags available"
        : "Select the Reply tag";

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
      <GroupedSettingsStack className="qotd-grouped-stack">
        <GroupedSettingsSection>
          <GroupedSettingsCopy>
            <p className="section-label">Settings</p>
            <GroupedSettingsHeading as="h2" variant="section">
              Workflow settings
            </GroupedSettingsHeading>
          </GroupedSettingsCopy>

          {channelOptions.notice ? (
            <GroupedSettingsInlineMessage
              message={channelOptions.notice.message}
              tone="error"
              action={
                <button
                  className="button-secondary"
                  type="button"
                  disabled={channelOptions.loading}
                  onClick={() => void channelOptions.refresh()}
                >
                  Retry channel lookup
                </button>
              }
            />
          ) : null}

          <GroupedSettingsGroup>
            <GroupedSettingsItem stacked role="group" aria-labelledby={workflowHeadingId}>
              <GroupedSettingsSubrow>
                <GroupedSettingsMainRow>
                  <GroupedSettingsCopy>
                    <GroupedSettingsHeading id={workflowHeadingId}>
                      Enable QOTD workflow
                    </GroupedSettingsHeading>
                  </GroupedSettingsCopy>
                  <GroupedSettingsSwitch
                    label="Enable QOTD workflow"
                    checked={draft.enabled}
                    disabled={controlsDisabled}
                    onChange={(checked) =>
                      setDraft((current) => ({
                        ...current,
                        enabled: checked,
                      }))
                    }
                  />
                </GroupedSettingsMainRow>
              </GroupedSettingsSubrow>

              <GroupedSettingsSubrow>
                <div className="qotd-settings-fields">
                  <SettingsSelectField
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
                    placeholder={forumChannelPlaceholder}
                    disabled={controlsDisabled || channelOptions.loading}
                    note="Forum used for official posts and reply threads."
                  />

                  <SettingsSelectField
                    label="Question tag"
                    value={draft.question_tag_id}
                    onChange={(value) =>
                      setDraft((current) => ({
                        ...current,
                        question_tag_id: value,
                      }))
                    }
                    options={tagOptions}
                    placeholder={questionTagPlaceholder}
                    disabled={controlsDisabled || !tagLookupAvailable || refreshingTags}
                    note="Tag for the official QOTD post."
                  />

                  <SettingsSelectField
                    label="Reply tag"
                    value={draft.reply_tag_id}
                    onChange={(value) =>
                      setDraft((current) => ({
                        ...current,
                        reply_tag_id: value,
                      }))
                    }
                    options={tagOptions}
                    placeholder={replyTagPlaceholder}
                    disabled={controlsDisabled || !tagLookupAvailable || refreshingTags}
                    note="Tag for member reply threads."
                  />
                </div>
              </GroupedSettingsSubrow>

              {!tagLookupAvailable ? (
                <GroupedSettingsSubrow>
                  <GroupedSettingsInlineMessage
                    message="Choose a forum channel before selecting tags."
                    tone="info"
                  />
                </GroupedSettingsSubrow>
              ) : refreshingTags ? (
                <GroupedSettingsSubrow>
                  <GroupedSettingsInlineMessage
                    message="Loading forum tags for the selected forum."
                    tone="info"
                  />
                </GroupedSettingsSubrow>
              ) : null}
            </GroupedSettingsItem>
          </GroupedSettingsGroup>
        </GroupedSettingsSection>
      </GroupedSettingsStack>

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
  };
}

function settingsDraftChanged(previous: Partial<SettingsDraft> | QOTDConfig, next: SettingsDraft) {
  const normalizedPrevious = createSettingsDraft(previous);
  return (
    normalizedPrevious.enabled !== next.enabled ||
    normalizedPrevious.forum_channel_id !== next.forum_channel_id ||
    normalizedPrevious.question_tag_id !== next.question_tag_id ||
    normalizedPrevious.reply_tag_id !== next.reply_tag_id
  );
}
