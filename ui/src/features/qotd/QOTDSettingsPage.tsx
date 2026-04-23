import { useEffect, useId, useRef, useState } from "react";
import type {
  QOTDCollectorConfig,
  QOTDConfig,
  QOTDDeck,
  QOTDDeckSummary,
} from "../../api/control";
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
import { useGuildRoleOptions } from "../features/useGuildRoleOptions";
import { QOTD_BUSY_LABELS, useQOTD } from "./QOTDContext";

interface SettingsDraft {
  active_deck_id: string;
  verified_role_id?: string;
  decks: QOTDDeck[];
  collector?: QOTDCollectorConfig;
}

export function QOTDSettingsPage() {
  const { canEditSelectedGuild } = useDashboardSession();
  const { busyLabel, deckSummaries, saveSettings, settings, setupChannel } = useQOTD();
  const channelOptions = useGuildChannelOptions();
  const roleOptions = useGuildRoleOptions();
  const workflowHeadingId = useId();
  const savedDraftRef = useRef<SettingsDraft>(createSettingsDraft(settings));
  const [draft, setDraft] = useState<SettingsDraft>(
    () => savedDraftRef.current,
  );
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

  const textChannelOptions = channelOptions.channels
    .filter((channel) => channel.kind === "text")
    .map((channel) => ({
      value: channel.id,
      label: channel.display_name,
      description: "Text channel that hosts the daily QOTD message and its answer thread.",
    }));
  const hasUnsavedChanges = settingsDraftChanged(savedDraftRef.current, draft);
  const setupBusy = busyLabel === QOTD_BUSY_LABELS.setupChannel;
  const controlsDisabled = !canEditSelectedGuild || saving || setupBusy;
  const activeDeckDraft = draft.decks.find((deck) => deck.id === draft.active_deck_id);
  const hasConfiguredChannel =
    (activeDeckDraft?.channel_id ?? "").trim() !== "";
  const setupDisabled =
    !canEditSelectedGuild ||
    saving ||
    setupBusy ||
    hasUnsavedChanges;
  const channelPlaceholder = channelOptions.loading
    ? "Loading text channels..."
    : textChannelOptions.length === 0
      ? "No text channels available"
      : "Select a text channel";

  async function handleSave() {
    if (controlsDisabled) {
      return;
    }

    setSaving(true);
    try {
      const updatedSettings = await saveSettings(draft);
      if (updatedSettings != null) {
        const nextDraft = createSettingsDraft(updatedSettings);
        savedDraftRef.current = nextDraft;
        setDraft(nextDraft);
      }
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
            <GroupedSettingsItem
              stacked
              role="group"
              aria-labelledby={`${workflowHeadingId}-setup`}
            >
              <GroupedSettingsSubrow>
                <GroupedSettingsMainRow>
                  <GroupedSettingsCopy>
                    <GroupedSettingsHeading id={`${workflowHeadingId}-setup`}>
                      Automatic setup
                    </GroupedSettingsHeading>
                    <p className="field-note">
                      Creates or repairs the <code>☆-qotd-☆</code> text channel with verified-role permissions so each daily QOTD post can open its own answer thread.
                    </p>
                  </GroupedSettingsCopy>
                  <button
                    className="button-primary"
                    type="button"
                    disabled={setupDisabled}
                    onClick={() => {
                      void (async () => {
                        await setupChannel(draft.active_deck_id);
                        await channelOptions.refresh();
                      })().catch(() => undefined);
                    }}
                  >
                    {setupBusy
                      ? "Setting up..."
                      : hasConfiguredChannel
                        ? "Repair QOTD setup"
                        : "Create QOTD channel"}
                  </button>
                </GroupedSettingsMainRow>
              </GroupedSettingsSubrow>

              {hasUnsavedChanges ? (
                <GroupedSettingsSubrow>
                  <GroupedSettingsInlineMessage
                    message="Save the current deck changes before running automatic setup."
                    tone="info"
                  />
                </GroupedSettingsSubrow>
              ) : null}
            </GroupedSettingsItem>

            <GroupedSettingsItem
              stacked
              role="group"
              aria-labelledby={workflowHeadingId}
            >
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

          <GroupedSettingsGroup>
            <GroupedSettingsItem>
              <GroupedSettingsSubrow>
                <div className="input-group">
                  <label>
                    <GroupedSettingsHeading>Verified Role</GroupedSettingsHeading>
                    <span className="field-note">The role required to view the QOTD text channels.</span>
                  </label>
                  <SettingsSelectField
                    label="Verified role"
                    value={draft.verified_role_id ?? ""}
                    onChange={(value) =>
                      setDraft((current) => ({
                        ...current,
                        verified_role_id: value,
                      }))
                    }
                    options={roleOptions.roles.map(r => ({ value: r.id, label: r.name }))}
                    placeholder="Select a verified role (optional)"
                    disabled={controlsDisabled || roleOptions.loading}
                    note="Users without this role will not be able to read the QOTD text channel."
                  />
                </div>
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
                        <GroupedSettingsHeading
                          id={`${workflowHeadingId}-${deck.id}`}
                        >
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
                              entry.id === deck.id
                                ? { ...entry, enabled: checked }
                                : entry,
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
                        label="QOTD text channel"
                        value={deck.channel_id ?? ""}
                        onChange={(value) =>
                          setDraft((current) => ({
                            ...current,
                            decks: current.decks.map((entry) =>
                              entry.id === deck.id
                                ? {
                                    ...entry,
                                    channel_id: value,
                                  }
                                : entry,
                            ),
                          }))
                        }
                        options={textChannelOptions}
                        placeholder={channelPlaceholder}
                        disabled={controlsDisabled || channelOptions.loading}
                        note="Single text channel used for the daily QOTD post; each post opens a dedicated answer thread."
                      />
                    </div>
                  </GroupedSettingsSubrow>

                  <GroupedSettingsSubrow>
                    <div className="qotd-deck-card-footer">
                      <div className="qotd-deck-summary">
                        <span>
                          {summary?.cards_remaining ?? 0} cards remaining
                        </span>
                        <span>{summary?.counts.used ?? 0} used</span>
                        <span>
                          {draft.active_deck_id === deck.id
                            ? "Active deck"
                            : "Inactive deck"}
                        </span>
                      </div>

                      <div className="inline-actions">
                        <button
                          className="button-secondary"
                          type="button"
                          disabled={
                            controlsDisabled || draft.active_deck_id === deck.id
                          }
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
                                  ? (nextDecks[0]?.id ?? "")
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
                        collector: current.collector,
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

          {textChannelOptions.length === 0 && !channelOptions.loading ? (
            <GroupedSettingsInlineMessage
              message="Create or expose at least one text channel to configure QOTD delivery."
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
          channel_id: String(deck.channel_id ?? ""),
        }))
      : [createDeckDraft([])];
  const activeDeckID =
    String(settings.active_deck_id ?? "").trim() || String(decks[0]?.id ?? "");
  return {
    active_deck_id: activeDeckID,
    verified_role_id: String(settings.verified_role_id ?? "").trim(),
    decks,
    collector: normalizeCollectorConfig(settings.collector),
  };
}

function settingsDraftChanged(
  previous: QOTDConfig | SettingsDraft,
  next: SettingsDraft,
) {
  return (
    JSON.stringify(createSettingsDraft(previous as QOTDConfig)) !==
    JSON.stringify(next)
  );
}

function createDeckDraft(existingDecks: QOTDDeck[]): QOTDDeck {
  const suffix = existingDecks.length + 1;
  return {
    id: `deck-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`,
    name: `Deck ${suffix}`,
    enabled: false,
    channel_id: "",
  };
}

function normalizeCollectorConfig(
  collector?: QOTDCollectorConfig,
): QOTDCollectorConfig {
  return {
    source_channel_id: String(collector?.source_channel_id ?? "").trim(),
    author_ids: normalizeCollectorEntries(collector?.author_ids),
    title_patterns: normalizeCollectorEntries(collector?.title_patterns, {
      caseInsensitive: true,
    }),
    start_date: String(collector?.start_date ?? "").trim(),
  };
}

function normalizeCollectorEntries(
  values: readonly unknown[] | undefined,
  options: { caseInsensitive?: boolean } = {},
) {
  if (!Array.isArray(values) || values.length === 0) {
    return [];
  }

  const seen = new Set<string>();
  const normalized: string[] = [];
  for (const value of values) {
    const trimmed = String(value ?? "").trim();
    if (trimmed === "") {
      continue;
    }
    const key = options.caseInsensitive ? trimmed.toLowerCase() : trimmed;
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    normalized.push(trimmed);
  }
  return normalized;
}

function findDeckSummary(
  deckSummaries: QOTDDeckSummary[],
  deckID: string,
): QOTDDeckSummary | undefined {
  return deckSummaries.find((deck) => deck.id === deckID);
}
