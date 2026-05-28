import type { FeatureRecord, GuildRoleOption } from "../../api/control";
import { KeyValueList } from "../../components/ui";
import {
  formatRoleOptionLabel,
  summarizeAutoRoleSignal,
} from "../../features/features/roles";

export interface AutoRoleDrawerBodyProps {
  boosterRoleDraft: string;
  configEnabledDraft: string;
  levelRoleDraft: string;
  roleOptions: GuildRoleOption[];
  selectedFeature: FeatureRecord;
  targetRoleDraft: string;
  disabled?: boolean;
  setBoosterRoleDraft: (value: string) => void;
  setConfigEnabledDraft: (value: string) => void;
  setLevelRoleDraft: (value: string) => void;
  setTargetRoleDraft: (value: string) => void;
}

export function AutoRoleDrawerBody({
  boosterRoleDraft,
  configEnabledDraft,
  levelRoleDraft,
  roleOptions,
  selectedFeature,
  targetRoleDraft,
  disabled = false,
  setBoosterRoleDraft,
  setConfigEnabledDraft,
  setLevelRoleDraft,
  setTargetRoleDraft,
}: AutoRoleDrawerBodyProps) {
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
            value: summarizeAutoRoleSignal(selectedFeature),
          },
          {
            label: "Server setting",
            value:
              selectedFeature.override_state === "inherit"
                ? "Using default"
                : "Configured here",
          },
        ]}
      />

      <div className="field-grid roles-form-grid">
        <label className="field-stack">
          <span className="field-label">Assignment rule</span>
          <select
            aria-label="Assignment rule"
            disabled={disabled}
            value={configEnabledDraft}
            onChange={(event) => setConfigEnabledDraft(event.target.value)}
          >
            <option value="enabled">Enabled</option>
            <option value="disabled">Disabled</option>
          </select>
          <span className="meta-note">
            Leave the module on while pausing assignment.
          </span>
        </label>

        <label className="field-stack">
          <span className="field-label">Target role</span>
          <select
            aria-label="Target role"
            disabled={disabled}
            value={targetRoleDraft}
            onChange={(event) => setTargetRoleDraft(event.target.value)}
          >
            <option value="">Select a target role</option>
            {roleOptions.map((role) => (
              <option key={role.id} value={role.id}>
                {formatRoleOptionLabel(role)}
              </option>
            ))}
          </select>
        </label>

        <label className="field-stack">
          <span className="field-label">Level role</span>
          <select
            aria-label="Level role"
            disabled={disabled}
            value={levelRoleDraft}
            onChange={(event) => setLevelRoleDraft(event.target.value)}
          >
            <option value="">Select the level role</option>
            {roleOptions.map((role) => (
              <option key={role.id} value={role.id}>
                {formatRoleOptionLabel(role)}
              </option>
            ))}
          </select>
        </label>

        <label className="field-stack">
          <span className="field-label">Booster role</span>
          <select
            aria-label="Booster role"
            disabled={disabled}
            value={boosterRoleDraft}
            onChange={(event) => setBoosterRoleDraft(event.target.value)}
          >
            <option value="">Select the booster role</option>
            {roleOptions.map((role) => (
              <option key={role.id} value={role.id}>
                {formatRoleOptionLabel(role)}
              </option>
            ))}
          </select>
        </label>
      </div>
    </>
  );
}
