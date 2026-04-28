import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  ControlApiClient,
  QOTDConfig,
  QOTDSummary,
} from "../../api/control";
import { QOTDProvider, useQOTD } from "./QOTDContext";

const clientMock = {
  getQOTDSettings: vi.fn(),
  getQOTDSummary: vi.fn(),
  updateQOTDSettings: vi.fn(),
} satisfies Pick<
  ControlApiClient,
  | "getQOTDSettings"
  | "getQOTDSummary"
  | "updateQOTDSettings"
>;

const dashboardSessionMock = {
  authState: "signed_in",
  canEditSelectedGuild: true,
  canReadSelectedGuild: true,
  client: clientMock,
  selectedGuildID: "guild-1",
};

vi.mock("../../context/DashboardSessionContext", () => ({
  useDashboardSession: () => dashboardSessionMock,
}));

describe("QOTDContext", () => {
  beforeEach(() => {
    clientMock.getQOTDSettings.mockReset().mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      settings: createSettings(),
    });
    clientMock.getQOTDSummary.mockReset()
      .mockResolvedValueOnce({
        status: "ok",
        guild_id: "guild-1",
        summary: createSummary(3),
      })
      .mockResolvedValueOnce({
        status: "ok",
        guild_id: "guild-1",
        summary: createSummary(2),
      });
    clientMock.updateQOTDSettings.mockReset().mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      settings: createSettings(),
    });
  });

  it("refreshes shared workspace state from settings-only data", async () => {
    const user = userEvent.setup();

    render(
      <QOTDProvider>
        <QOTDContextHarness />
      </QOTDProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("3 cards remaining")).toBeInTheDocument();
    });

    const initialSettingsCalls = clientMock.getQOTDSettings.mock.calls.length;
    const initialSummaryCalls = clientMock.getQOTDSummary.mock.calls.length;

    await user.click(screen.getByRole("button", { name: "Refresh workspace" }));

    await waitFor(() => {
      expect(screen.getByText("2 cards remaining")).toBeInTheDocument();
    });
    expect(clientMock.getQOTDSettings.mock.calls.length).toBeGreaterThan(
      initialSettingsCalls,
    );
    expect(clientMock.getQOTDSummary.mock.calls.length).toBeGreaterThan(
      initialSummaryCalls,
    );
  });
});

function QOTDContextHarness() {
  const { deckSummaries, refreshWorkspace } = useQOTD();

  return (
    <div>
      <span>{deckSummaries[0]?.cards_remaining ?? 0} cards remaining</span>
      <button
        type="button"
        onClick={() => void refreshWorkspace()}
      >
        Refresh workspace
      </button>
    </div>
  );
}

function createSettings(): QOTDConfig {
  return {
    active_deck_id: "default",
    collector: {
      source_channel_id: "collector-channel-1",
      author_ids: ["bot-1"],
      title_patterns: ["Question Of The Day"],
    },
    decks: [
      {
        id: "default",
        name: "Default",
        enabled: true,
        channel_id: "question-channel-1",
      },
    ],
  };
}

function createSummary(cardsRemaining: number): QOTDSummary {
  const total = cardsRemaining;
  return {
    settings: createSettings(),
    counts: {
      total,
      draft: 0,
      ready: total,
      reserved: 0,
      used: 0,
      disabled: 0,
    },
    decks: [
      {
        id: "default",
        name: "Default",
        enabled: true,
        counts: {
          total,
          draft: 0,
          ready: total,
          reserved: 0,
          used: 0,
          disabled: 0,
        },
        cards_remaining: total,
        is_active: true,
        can_publish: true,
      },
    ],
    current_publish_date_utc: "2026-04-04T00:00:00Z",
    published_for_current_slot: false,
  };
}
