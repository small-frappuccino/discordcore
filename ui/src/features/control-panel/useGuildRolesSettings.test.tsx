import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import {
  clearGuildRolesSettingsCache,
  useGuildRolesSettings,
} from "./useGuildRolesSettings";

let mockDashboardSession: {
  authState: string;
  baseUrl: string;
  client: {
    getGuildSettings: ReturnType<typeof vi.fn>;
  };
  selectedGuildID: string;
};

vi.mock("../../context/DashboardSessionContext", () => ({
  useDashboardSession: () => mockDashboardSession,
}));

describe("useGuildRolesSettings", () => {
  beforeEach(() => {
    mockDashboardSession = {
      authState: "signed_in",
      baseUrl: "https://control.example.test",
      client: {
        getGuildSettings: vi.fn().mockResolvedValue({
          workspace: {
            sections: {
              bot_routing: {
                bot_instance_id: "main",
                available_bot_instance_ids: ["main", "companion"],
                domain_bot_instance_ids: {
                  qotd: "companion",
                },
                editable_domains: ["qotd"],
              },
              roles: {
                allowed: [],
                dashboard_read: [],
                dashboard_write: [],
              },
            },
          },
        }),
      },
      selectedGuildID: "guild-1",
    };
  });

  afterEach(() => {
    clearGuildRolesSettingsCache();
    vi.clearAllMocks();
  });

  it("reuses cached empty dashboard role settings without returning to loading", async () => {
    const firstHook = renderHook(() => useGuildRolesSettings());

    await waitFor(() => {
      expect(
        mockDashboardSession.client.getGuildSettings,
      ).toHaveBeenCalledTimes(1);
    });
    await waitFor(() => {
      expect(firstHook.result.current.loading).toBe(false);
    });

    expect(firstHook.result.current.roles).toEqual({
      allowedRoleIds: [],
      dashboardReadRoleIds: [],
      dashboardWriteRoleIds: [],
    });
    expect(firstHook.result.current.botRouting).toEqual({
      botInstanceID: "main",
      availableBotInstanceIDs: ["main", "companion"],
      domainBotInstanceIDs: {
        qotd: "companion",
      },
      editableDomains: ["qotd"],
    });

    firstHook.unmount();

    const secondHook = renderHook(() => useGuildRolesSettings());

    expect(secondHook.result.current.loading).toBe(false);
    expect(secondHook.result.current.roles).toEqual({
      allowedRoleIds: [],
      dashboardReadRoleIds: [],
      dashboardWriteRoleIds: [],
    });
    expect(secondHook.result.current.botRouting).toEqual({
      botInstanceID: "main",
      availableBotInstanceIDs: ["main", "companion"],
      domainBotInstanceIDs: {
        qotd: "companion",
      },
      editableDomains: ["qotd"],
    });

    await waitFor(() => {
      expect(
        mockDashboardSession.client.getGuildSettings,
      ).toHaveBeenCalledTimes(1);
    });
  });
});
