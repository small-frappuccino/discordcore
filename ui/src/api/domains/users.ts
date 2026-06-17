import { ControlApiClient } from "../client";

export interface UserPreferences {
  theme: string;
  timezone: string;
}

export async function getUserPreferences(
  client: ControlApiClient,
): Promise<UserPreferences> {
  return client.request<UserPreferences>("GET", "/v1/users/@me/preferences");
}

export async function updateUserPreferences(
  client: ControlApiClient,
  preferences: UserPreferences,
): Promise<UserPreferences> {
  return client.request<UserPreferences>(
    "PUT",
    "/v1/users/@me/preferences",
    preferences,
  );
}
