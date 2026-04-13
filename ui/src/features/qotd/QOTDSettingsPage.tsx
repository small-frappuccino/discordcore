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
import { useQOTD } from "./QOTDContext";

interface SettingsDraft {
  enabled: boolean;
  question_channel_id: string;
  response_channel_id: string;
}

export function QOTDSettingsPage() {
  const { canEditSelectedGuild } = useDashboardSession();
  const { saveSettings, settings } = useQOTD();
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

  const messageChannelOptions = channelOptions.channels
    .filter((channel) => channel.supports_message_route)
    .map((channel) => ({
      value: channel.id,
      label: channel.display_name,
      description:
        channel.kind === "announcement"
          ? "Announcement channel that accepts QOTD message posts."
          : "Text channel that accepts QOTD message posts.",
    }));
  const hasUnsavedChanges = settingsDraftChanged(savedDraftRef.current, draft);
  const controlsDisabled = !canEditSelectedGuild || saving;
  const channelPlaceholder = channelOptions.loading
    ? "Loading message channels..."
    : messageChannelOptions.length === 0
      ? "No message channels available"
      : "Select a channel";

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
    setDraft(savedDraftRef.current);
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
                    label="Question channel"
                    value={draft.question_channel_id}
                    onChange={(value) =>
                      setDraft((current) => ({
                        ...current,
                        question_channel_id: value,
                      }))
                    }
                    options={messageChannelOptions}
                    placeholder={channelPlaceholder}
                    disabled={controlsDisabled || channelOptions.loading}
                    note="Read-only channel where the daily QOTD prompt is posted."
                  />

                  <SettingsSelectField
                    label="Response channel"
                    value={draft.response_channel_id}
                    onChange={(value) =>
                      setDraft((current) => ({
                        ...current,
                        response_channel_id: value,
                      }))
                    }
                    options={messageChannelOptions}
                    placeholder={channelPlaceholder}
                    disabled={controlsDisabled || channelOptions.loading}
                    note="Channel where each member answer is posted as an embed."
                  />
                </div>
              </GroupedSettingsSubrow>

              {messageChannelOptions.length === 0 && !channelOptions.loading ? (
                <GroupedSettingsSubrow>
                  <GroupedSettingsInlineMessage
                    message="Create or expose at least one text or announcement channel to configure QOTD delivery."
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
    question_channel_id: String(settings.question_channel_id ?? ""),
    response_channel_id: String(settings.response_channel_id ?? ""),
  };
}

function settingsDraftChanged(previous: Partial<SettingsDraft> | QOTDConfig, next: SettingsDraft) {
  const normalizedPrevious = createSettingsDraft(previous);
  return (
    normalizedPrevious.enabled !== next.enabled ||
    normalizedPrevious.question_channel_id !== next.question_channel_id ||
    normalizedPrevious.response_channel_id !== next.response_channel_id
  );
}
