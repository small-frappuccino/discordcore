import type { ReactNode } from "react";
import { renderHook, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  PartnerBoardProvider,
  usePartnerBoard,
} from "./PartnerBoardContext";
import { resetPartnerBoardCache } from "./cache";

let mockDashboardSession: {
  authState: string;
  baseUrl: string;
  canEditSelectedGuild: boolean;
  canReadSelectedGuild: boolean;
  client: {
    getPartnerBoard: ReturnType<typeof vi.fn>;
  };
  selectedGuildID: string;
};

vi.mock("../../context/DashboardSessionContext", () => ({
  useDashboardSession: () => mockDashboardSession,
}));

describe("PartnerBoardProvider", () => {
  beforeEach(() => {
    mockDashboardSession = {
      authState: "signed_in",
      baseUrl: "https://control.example.test",
      canEditSelectedGuild: true,
      canReadSelectedGuild: true,
      client: {
        getPartnerBoard: vi.fn().mockResolvedValue({
          partner_board: {
            target: {
              type: "channel_message",
              channel_id: "channel-1",
              message_id: "message-1",
            },
            template: {
              title: "Partner Board",
              section_header_template: "{group}",
              line_template: "{name}",
              empty_state_text: "No partners yet",
            },
            partners: [
              {
                name: "Alpha",
                link: "https://discord.gg/alpha",
              },
            ],
          },
        }),
      },
      selectedGuildID: "guild-1",
    };
  });

  afterEach(() => {
    resetPartnerBoardCache();
    vi.clearAllMocks();
  });

  it("reuses cached board data without dropping back to loading on remount", async () => {
    const wrapper = ({ children }: { children: ReactNode }) => (
      <PartnerBoardProvider>{children}</PartnerBoardProvider>
    );

    const firstHook = renderHook(() => usePartnerBoard(), {
      wrapper,
    });

    await waitFor(() => {
      expect(firstHook.result.current.workspaceState).toBe("ready");
    });

    expect(firstHook.result.current.partners).toHaveLength(1);
    expect(mockDashboardSession.client.getPartnerBoard).toHaveBeenCalledTimes(1);

    firstHook.unmount();

    const secondHook = renderHook(() => usePartnerBoard(), {
      wrapper,
    });

    expect(secondHook.result.current.loading).toBe(false);
    expect(secondHook.result.current.workspaceState).toBe("ready");
    expect(secondHook.result.current.partners).toHaveLength(1);

    await waitFor(() => {
      expect(mockDashboardSession.client.getPartnerBoard).toHaveBeenCalledTimes(2);
    });
  });
});
