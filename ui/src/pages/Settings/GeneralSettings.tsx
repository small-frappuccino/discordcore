import { useUserPreferences } from "../../context/UserPreferencesContext";
import { SettingsGroup, SettingsRow, SelectMenu } from "../../components/ui/tahoe";

export function GeneralSettings() {
  const { preferences, updatePreferences, isUpdating } = useUserPreferences();

  const handleThemeChange = (value: string) => {
    if (!preferences) return;
    updatePreferences({ ...preferences, theme: value });
  };

  const handleTimezoneChange = (value: string) => {
    if (!preferences) return;
    updatePreferences({ ...preferences, timezone: value });
  };

  return (
    <div>
      <h2 className="text-2xl font-bold mb-2 text-text-primary">Account</h2>
      <p className="text-sm text-text-secondary mb-8">
        Manage your plan, credentials, and general preferences.
      </p>

      <h3 className="text-sm font-bold text-text-primary mb-3">General</h3>
      <SettingsGroup>
        <SettingsRow
          title="Theme"
          description="Customize the appearance of the dashboard."
          control={
            <SelectMenu
              value={preferences?.theme || "system"}
              options={[
                { label: "System Default", value: "system" },
                { label: "Dark", value: "dark" },
                { label: "Light", value: "light" }
              ]}
              onChange={handleThemeChange}
              disabled={isUpdating}
            />
          }
        />
        <SettingsRow
          title="Timezone"
          description="Adjust the timezone used for displaying times across the dashboard."
          control={
            <SelectMenu
              value={preferences?.timezone || "UTC"}
              options={[
                { label: "UTC", value: "UTC" },
                { label: "America/New_York", value: "America/New_York" },
                { label: "America/Los_Angeles", value: "America/Los_Angeles" },
                { label: "Europe/London", value: "Europe/London" },
                { label: "Asia/Tokyo", value: "Asia/Tokyo" }
              ]}
              onChange={handleTimezoneChange}
              disabled={isUpdating}
            />
          }
        />
      </SettingsGroup>
    </div>
  );
}
