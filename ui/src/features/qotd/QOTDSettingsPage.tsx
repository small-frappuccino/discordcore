import { useEffect, useId, useRef, useState } from "react";
import type { QOTDConfig, QOTDDeck, QOTDDeckSummary } from "../../api/control";
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
  active_deck_id: string;
  decks: QOTDDeck[];
}

export function QOTDSettingsPage() {
  const { canEditSelectedGuild } = useDashboardSession();
  const { deckSummaries, saveSettings, settings } = useQOTD();
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
                      Active publishing deck
                    </GroupedSettingsHeading>
                  </GroupedSettingsCopy>
                </GroupedSettingsMainRow>
              </GroupedSettingsSubrow>

              <GroupedSettingsSubrow>
                <SettingsSelectField
                  label="Active deck"
                  value={draft.active_deck_id}
                  onChange={(value) =>
                    setDraft((current) => ({
                      ...current,
                      active_deck_id: value,
                    }))
                  }
                  options={draft.decks.map((deck) => ({
                    value: deck.id,
                    label: deck.name,
                    description: deck.enabled
                      ? "Deck can be used for scheduled/manual publishing."
                      : "Deck exists, but publishing is disabled.",
                  }))}
                  placeholder="Select a deck"
                  disabled={controlsDisabled || draft.decks.length === 0}
                  note="Manual and scheduled QOTD posts draw from this deck."
                />
              </GroupedSettingsSubrow>
            </GroupedSettingsItem>
          </GroupedSettingsGroup>
        </GroupedSettingsSection>

        <GroupedSettingsSection>
          <GroupedSettingsCopy>
            <p className="section-label">Decks</p>
            <GroupedSettingsHeading as="h2" variant="section">
              Manage decks
            </GroupedSettingsHeading>
          </GroupedSettingsCopy>

          <GroupedSettingsGroup>
            {draft.decks.map((deck) => {
              const summary = findDeckSummary(deckSummaries, deck.id);
              const questionCount = summary?.counts.total ?? 0;
              const canDelete = draft.decks.length > 1;

              return (
                <GroupedSettingsItem
                  key={deck.id}
                  stacked
                  role="group"
                  aria-labelledby={`${workflowHeadingId}-${deck.id}`}
                >
                  <GroupedSettingsSubrow>
                    <GroupedSettingsMainRow>
                      <GroupedSettingsCopy>
                        <GroupedSettingsHeading id={`${workflowHeadingId}-${deck.id}`}>
                          {deck.name}
                        </GroupedSettingsHeading>
                      </GroupedSettingsCopy>
                      <GroupedSettingsSwitch
                        label={`Enable ${deck.name}`}
                        checked={Boolean(deck.enabled)}
                        disabled={controlsDisabled}
                        onChange={(checked) =>
                          setDraft((current) => ({
                            ...current,
                            decks: current.decks.map((entry) =>
                              entry.id === deck.id ? { ...entry, enabled: checked } : entry,
                            ),
                          }))
                        }
                      />
                    </GroupedSettingsMainRow>
                  </GroupedSettingsSubrow>

                  <GroupedSettingsSubrow>
                    <div className="qotd-settings-fields">
                      <label className="field-stack">
                        <span className="field-label">Deck name</span>
                        <input
                          type="text"
                          value={deck.name}
                          disabled={controlsDisabled}
                          onChange={(event) =>
                            setDraft((current) => ({
                              ...current,
                              decks: current.decks.map((entry) =>
                                entry.id === deck.id
                                  ? { ...entry, name: event.target.value }
                                  : entry,
                              ),
                            }))
                          }
                        />
                      </label>

                      <SettingsSelectField
                        label="Question channel"
                        value={deck.question_channel_id ?? ""}
                        onChange={(value) =>
                          setDraft((current) => ({
                            ...current,
                            decks: current.decks.map((entry) =>
                              entry.id === deck.id
                                ? { ...entry, question_channel_id: value }
                                : entry,
                            ),
                          }))
                        }
                        options={messageChannelOptions}
                        placeholder={channelPlaceholder}
                        disabled={controlsDisabled || channelOptions.loading}
                        note="Read-only channel where the deck prompt is posted."
                      />

                      <SettingsSelectField
                        label="Response channel"
                        value={deck.response_channel_id ?? ""}
                        onChange={(value) =>
                          setDraft((current) => ({
                            ...current,
                            decks: current.decks.map((entry) =>
                              entry.id === deck.id
                                ? { ...entry, response_channel_id: value }
                                : entry,
                            ),
                          }))
                        }
                        options={messageChannelOptions}
                        placeholder={channelPlaceholder}
                        disabled={controlsDisabled || channelOptions.loading}
                        note="Channel where answers for this deck are posted."
                      />
                    </div>
                  </GroupedSettingsSubrow>

                  <GroupedSettingsSubrow>
                    <div className="qotd-deck-card-footer">
                      <div className="qotd-deck-summary">
                        <span>{summary?.cards_remaining ?? 0} cards remaining</span>
                        <span>{summary?.counts.used ?? 0} used</span>
                        <span>
                          {draft.active_deck_id === deck.id ? "Active deck" : "Inactive deck"}
                        </span>
                      </div>

                      <div className="inline-actions">
                        <button
                          className="button-secondary"
                          type="button"
                          disabled={controlsDisabled || draft.active_deck_id === deck.id}
                          onClick={() =>
                            setDraft((current) => ({
                              ...current,
                              active_deck_id: deck.id,
                            }))
                          }
                        >
                          Set active
                        </button>
                        <button
                          className="button-secondary"
                          type="button"
                          disabled={controlsDisabled || !canDelete}
                          onClick={() =>
                            setDraft((current) => {
                              const nextDecks = current.decks.filter(
                                (entry) => entry.id !== deck.id,
                              );
                              const nextActiveDeckID =
                                current.active_deck_id === deck.id
                                  ? nextDecks[0]?.id ?? ""
                                  : current.active_deck_id;
                              return {
                                ...current,
                                active_deck_id: nextActiveDeckID,
                                decks: nextDecks,
                              };
                            })
                          }
                        >
                          Delete deck
                        </button>
                      </div>
                    </div>
                  </GroupedSettingsSubrow>

                  {!canDelete ? (
                    <GroupedSettingsSubrow>
                      <GroupedSettingsInlineMessage
                        message="At least one deck must remain configured."
                        tone="info"
                      />
                    </GroupedSettingsSubrow>
                  ) : questionCount > 0 ? (
                    <GroupedSettingsSubrow>
                      <GroupedSettingsInlineMessage
                        message={
                          questionCount === 1
                            ? "Deleting this deck also removes 1 question from this bank."
                            : `Deleting this deck also removes ${questionCount} questions from this bank.`
                        }
                        tone="info"
                      />
                    </GroupedSettingsSubrow>
                  ) : null}
                </GroupedSettingsItem>
              );
            })}
          </GroupedSettingsGroup>

          <GroupedSettingsGroup>
            <GroupedSettingsItem>
              <GroupedSettingsSubrow>
                <button
                  className="button-secondary"
                  type="button"
                  disabled={controlsDisabled}
                  onClick={() =>
                    setDraft((current) => {
                      const newDeck = createDeckDraft(current.decks);
                      return {
                        active_deck_id: current.active_deck_id || newDeck.id,
                        decks: [...current.decks, newDeck],
                      };
                    })
                  }
                >
                  Add deck
                </button>
              </GroupedSettingsSubrow>
            </GroupedSettingsItem>
          </GroupedSettingsGroup>

          {messageChannelOptions.length === 0 && !channelOptions.loading ? (
            <GroupedSettingsInlineMessage
              message="Create or expose at least one text or announcement channel to configure QOTD delivery."
              tone="info"
            />
          ) : null}
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

function createSettingsDraft(settings: QOTDConfig): SettingsDraft {
  const decks =
    Array.isArray(settings.decks) && settings.decks.length > 0
      ? settings.decks.map((deck) => ({
          id: String(deck.id ?? ""),
          name: String(deck.name ?? ""),
          enabled: Boolean(deck.enabled),
          question_channel_id: String(deck.question_channel_id ?? ""),
          response_channel_id: String(deck.response_channel_id ?? ""),
        }))
      : [createDeckDraft([])];
  const activeDeckID =
    String(settings.active_deck_id ?? "").trim() || String(decks[0]?.id ?? "");
  return {
    active_deck_id: activeDeckID,
    decks,
  };
}

function settingsDraftChanged(previous: QOTDConfig | SettingsDraft, next: SettingsDraft) {
  return JSON.stringify(createSettingsDraft(previous as QOTDConfig)) !== JSON.stringify(next);
}

function createDeckDraft(existingDecks: QOTDDeck[]): QOTDDeck {
  const suffix = existingDecks.length + 1;
  return {
    id: `deck-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`,
    name: `Deck ${suffix}`,
    enabled: false,
    question_channel_id: "",
    response_channel_id: "",
  };
}

function findDeckSummary(
  deckSummaries: QOTDDeckSummary[],
  deckID: string,
): QOTDDeckSummary | undefined {
  return deckSummaries.find((deck) => deck.id === deckID);
}
