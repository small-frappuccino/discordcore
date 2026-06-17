/* eslint-disable react-refresh/only-export-components */
import {
  createContext,
  useContext,
  useMemo,
  useCallback,
  type ReactNode,
  useEffect,
} from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useDashboardSession } from "./DashboardSessionContext";
import {
  getUserPreferences,
  updateUserPreferences,
  type UserPreferences,
} from "../api/domains/users";
import toast from "react-hot-toast";

interface UserPreferencesContextValue {
  preferences: UserPreferences | undefined;
  isLoading: boolean;
  isError: boolean;
  updatePreferences: (newPrefs: UserPreferences) => Promise<void>;
  isUpdating: boolean;
}

const UserPreferencesContext = createContext<UserPreferencesContextValue | null>(
  null,
);

const defaultPreferences: UserPreferences = {
  theme: "system",
  timezone: "UTC",
};

export function UserPreferencesProvider({ children }: { children: ReactNode }) {
  const { client, authState } = useDashboardSession();
  const queryClient = useQueryClient();

  const {
    data: preferences,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ["user-preferences", client.getBaseUrl()],
    queryFn: () => getUserPreferences(client),
    enabled: authState === "signed_in",
    staleTime: 1000 * 60 * 60, // 1 hour
  });

  const mutation = useMutation({
    mutationFn: (newPrefs: UserPreferences) =>
      updateUserPreferences(client, newPrefs),
    onSuccess: (data) => {
      queryClient.setQueryData(["user-preferences", client.getBaseUrl()], data);
      toast.success("Preferences saved");
    },
    onError: () => {
      toast.error("Failed to save preferences");
    },
  });

  const updatePreferences = useCallback(
    async (newPrefs: UserPreferences) => {
      await mutation.mutateAsync(newPrefs);
    },
    [mutation.mutateAsync],
  );

  const contextValue = useMemo(
    () => ({
      preferences: preferences ?? defaultPreferences,
      isLoading,
      isError,
      updatePreferences,
      isUpdating: mutation.isPending,
    }),
    [preferences, isLoading, isError, updatePreferences, mutation.isPending],
  );

  // Apply theme class to document body
  useEffect(() => {
    if (typeof window === "undefined") return;

    const theme = preferences?.theme || "system";
    const isDark =
      theme === "dark" ||
      (theme === "system" &&
        window.matchMedia("(prefers-color-scheme: dark)").matches);

    if (isDark) {
      document.documentElement.classList.add("theme-dark");
      document.documentElement.classList.remove("theme-light");
    } else {
      document.documentElement.classList.add("theme-light");
      document.documentElement.classList.remove("theme-dark");
    }
  }, [preferences?.theme]);

  return (
    <UserPreferencesContext.Provider value={contextValue}>
      {children}
    </UserPreferencesContext.Provider>
  );
}

export function useUserPreferences() {
  const context = useContext(UserPreferencesContext);
  if (context === null) {
    throw new Error(
      "useUserPreferences must be used within a UserPreferencesProvider",
    );
  }
  return context;
}
