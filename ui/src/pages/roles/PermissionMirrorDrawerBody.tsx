import type { FeatureRecord, GuildRoleOption } from "../../api/control";
import { KeyValueList } from "../../components/ui";
import {
  formatRoleOptionLabel,
  formatRoleValue,
  getPermissionMirrorDetails,
  summarizeAdvancedRoleSignal,
} from "../../features/features/roles";

type PermissionMirrorDetails = ReturnType<typeof getPermissionMirrorDetails>;

export interface PermissionMirrorDrawerBodyProps {
  actorRoleDraft: string;
  permissionMirrorDetails: PermissionMirrorDetails;
  roleOptions: GuildRoleOption[];
  selectedFeature: FeatureRecord;
  disabled?: boolean;
  setActorRoleDraft: (value: string) => void;
}

export function PermissionMirrorDrawerBody({
  actorRoleDraft,
  permissionMirrorDetails,
  roleOptions,
  selectedFeature,
  disabled = false,
  setActorRoleDraft,
}: PermissionMirrorDrawerBodyProps) {
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
          {
            label: "Current actor role",
            value: formatRoleValue(
              permissionMirrorDetails.actorRoleId,
              roleOptions,
              "No guard role",
            ),
          },
        ]}
      />

      <label className="field-stack">
        <span className="field-label">Actor role</span>
        <select
          aria-label="Actor role"
          disabled={disabled}
          value={actorRoleDraft}
          onChange={(event) => setActorRoleDraft(event.target.value)}
        >
          <option value="">No guard role</option>
          {roleOptions.map((role) => (
            <option key={role.id} value={role.id}>
              {formatRoleOptionLabel(role)}
            </option>
          ))}
        </select>
        <span className="meta-note">Leave empty to keep the guard global.</span>
      </label>
    </>
  );
}
