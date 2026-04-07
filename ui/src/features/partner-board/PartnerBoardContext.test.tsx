import type { ReactNode } from "react";
import { act, renderHook, waitFor } from "@testing-library/react";
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
          partner_board: buildBoard(),
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

  it("preserves a dirty layout draft when a cached workspace receives a fresher snapshot", async () => {
    const wrapper = ({ children }: { children: ReactNode }) => (
      <PartnerBoardProvider>{children}</PartnerBoardProvider>
    );

    const firstHook = renderHook(() => usePartnerBoard(), {
      wrapper,
    });

    await waitFor(() => {
      expect(firstHook.result.current.workspaceState).toBe("ready");
    });
    firstHook.unmount();

    const pendingReload = createDeferred<{
      partner_board: ReturnType<typeof buildBoard>;
    }>();
    mockDashboardSession.client.getPartnerBoard.mockReset();
    mockDashboardSession.client.getPartnerBoard.mockReturnValue(pendingReload.promise);

    const secondHook = renderHook(() => usePartnerBoard(), {
      wrapper,
    });

    expect(secondHook.result.current.workspaceState).toBe("ready");
    expect(secondHook.result.current.layoutForm.title).toBe("Partner Board");

    act(() => {
      secondHook.result.current.setLayoutFormField("title", "Dirty local title");
    });

    expect(secondHook.result.current.layoutDirty).toBe(true);

    act(() => {
      pendingReload.resolve({
        partner_board: buildBoard({
          template: {
            title: "Fresh server title",
          },
        }),
      });
    });

    await waitFor(() => {
      expect(secondHook.result.current.board?.template?.title).toBe("Fresh server title");
    });

    expect(secondHook.result.current.layoutForm.title).toBe("Dirty local title");
    expect(secondHook.result.current.layoutDirty).toBe(true);
  });
});

function buildBoard(overrides: {
  partners?: Array<{
    fandom?: string;
    link?: string;
    name?: string;
  }>;
  target?: {
    type?: "channel_message" | "webhook_message";
    channel_id?: string;
    message_id?: string;
    webhook_url?: string;
  };
  template?: {
    title?: string;
    intro?: string;
    section_header_template?: string;
    line_template?: string;
    empty_state_text?: string;
  };
} = {}) {
  return {
    target: {
      type: "channel_message" as const,
      channel_id: "channel-1",
      message_id: "message-1",
      ...overrides.target,
    },
    template: {
      title: "Partner Board",
      intro: "",
      section_header_template: "{group}",
      line_template: "{name}",
      empty_state_text: "No partners yet",
      ...overrides.template,
    },
    partners: (overrides.partners ?? [
      {
        name: "Alpha",
        link: "https://discord.gg/alpha",
      },
    ]).map((partner) => ({
      fandom: partner.fandom,
      link: partner.link ?? "https://discord.gg/alpha",
      name: partner.name ?? "Alpha",
    })),
  };
}

function createDeferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise;
    reject = rejectPromise;
  });
  return {
    promise,
    resolve,
    reject,
  };
}
