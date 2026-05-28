import type { FeatureRecord } from "../../api/control";
import { KeyValueList } from "../../components/ui";
import { summarizeAdvancedRoleSignal } from "../../features/features/roles";

export interface PresenceWatchBotDrawerBodyProps {
  selectedFeature: FeatureRecord;
  watchBotDraft: string;
  disabled?: boolean;
  setWatchBotDraft: (value: string) => void;
}

export function PresenceWatchBotDrawerBody({
  selectedFeature,
  watchBotDraft,
  disabled = false,
  setWatchBotDraft,
}: PresenceWatchBotDrawerBodyProps) {
  return (
    <>
      <KeyValueList
        items={[
          {
            label: "Module state",
            value: selectedFeature.effective_enabled ? "On" : "Off",
          },
          {
            label: "Current signal",
            value: summarizeAdvancedRoleSignal(selectedFeature),
          },
        ]}
      />

      <label className="field-stack">
        <span className="field-label">Watch bot presence</span>
        <select
          aria-label="Watch bot presence"
          disabled={disabled}
          value={watchBotDraft}
          onChange={(event) => setWatchBotDraft(event.target.value)}
        >
          <option value="enabled">Enabled</option>
          <option value="disabled">Disabled</option>
        </select>
        <span className="meta-note">
          Controls whether the runtime watches the bot account.
        </span>
      </label>
    </>
  );
}
