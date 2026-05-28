import type { FeatureRecord, GuildMemberOption } from "../../api/control";
import { KeyValueList, LookupNotice } from "../../components/ui";
import {
  formatMemberOptionLabel,
  formatMemberValue,
  summarizeAdvancedRoleSignal,
} from "../../features/features/roles";

type DashboardNotice = {
  tone: "info" | "success" | "error";
  message: string;
};

export interface PresenceWatchUserDrawerBodyProps {
  memberLookupLoading: boolean;
  memberLookupNotice: DashboardNotice | null;
  memberOptions: GuildMemberOption[];
  memberSearchDraft: string;
  selectedFeature: FeatureRecord;
  userIdDraft: string;
  disabled?: boolean;
  refreshMemberOptions: () => Promise<void>;
  setMemberSearchDraft: (value: string) => void;
  setUserIdDraft: (value: string) => void;
}

export function PresenceWatchUserDrawerBody({
  memberLookupLoading,
  memberLookupNotice,
  memberOptions,
  memberSearchDraft,
  selectedFeature,
  userIdDraft,
  disabled = false,
  refreshMemberOptions,
  setMemberSearchDraft,
  setUserIdDraft,
}: PresenceWatchUserDrawerBodyProps) {
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
            label: "Current member",
            value: formatMemberValue(userIdDraft, memberOptions),
          },
        ]}
      />

      <div className="field-grid roles-form-grid">
        <label className="field-stack">
          <span className="field-label">Search members</span>
          <input
            aria-label="Search members"
            disabled={disabled}
            value={memberSearchDraft}
            onChange={(event) => setMemberSearchDraft(event.target.value)}
            placeholder="Search by username, nickname, or user ID"
          />
          <span className="meta-note">Filter the server member list.</span>
        </label>

        <label className="field-stack">
          <span className="field-label">Member</span>
          <select
            aria-label="Member"
            value={userIdDraft}
            disabled={
              disabled || memberLookupLoading || memberOptions.length === 0
            }
            onChange={(event) => setUserIdDraft(event.target.value)}
          >
            <option value="">
              {memberLookupLoading
                ? "Loading members..."
                : memberOptions.length === 0
                  ? "No matching members"
                  : "No member selected"}
            </option>
            {memberOptions.map((member) => (
              <option key={member.id} value={member.id}>
                {formatMemberOptionLabel(member)}
              </option>
            ))}
          </select>
          <span className="meta-note">
            Keep the current member or pick a new one.
          </span>
        </label>
      </div>

      {memberLookupNotice ? (
        <LookupNotice
          title="Member lookup unavailable"
          message={memberLookupNotice.message}
          retryDisabled={memberLookupLoading}
          retryLabel="Retry member lookup"
          onRetry={refreshMemberOptions}
        />
      ) : null}

      {!memberLookupNotice &&
      !memberLookupLoading &&
      memberOptions.length === 0 ? (
        <div className="surface-subsection">
          <p className="section-label">No matches</p>
          <p className="meta-note">
            Adjust the search text to find a different member from the selected
            server.
          </p>
        </div>
      ) : null}
    </>
  );
}
