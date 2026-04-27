import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  ControlApiClient,
  QOTDConfig,
  QOTDQuestion,
  QOTDSummary,
} from "../../api/control";
import { QOTDProvider, useQOTD } from "./QOTDContext";

const clientMock = {
  getQOTDSettings: vi.fn(),
  getQOTDSummary: vi.fn(),
  listQOTDQuestions: vi.fn(),
  removeQOTDCollectorDeckDuplicates: vi.fn(),
} satisfies Pick<
  ControlApiClient,
  | "getQOTDSettings"
  | "getQOTDSummary"
  | "listQOTDQuestions"
  | "removeQOTDCollectorDeckDuplicates"
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
    clientMock.listQOTDQuestions.mockReset()
      .mockResolvedValueOnce({
        status: "ok",
        guild_id: "guild-1",
        questions: createQuestions(3),
      })
      .mockResolvedValueOnce({
        status: "ok",
        guild_id: "guild-1",
        questions: createQuestions(2),
      });
    clientMock.removeQOTDCollectorDeckDuplicates.mockReset().mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      result: {
        deck_id: "default",
        scanned_messages: 8,
        matched_messages: 3,
        duplicate_questions: 1,
        deleted_questions: 1,
      },
    });
  });

  it("refreshes shared workspace state after collector duplicate removal", async () => {
    const user = userEvent.setup();

    render(
      <QOTDProvider>
        <QOTDContextHarness />
      </QOTDProvider>,
    );

    await waitFor(() => {
      expect(screen.getByText("3 cards remaining")).toBeInTheDocument();
    });
    expect(screen.getByText("3 questions loaded")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Remove duplicates" }));

    await waitFor(() => {
      expect(screen.getByText("1 deleted")).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getByText("2 cards remaining")).toBeInTheDocument();
    });
    expect(screen.getByText("2 questions loaded")).toBeInTheDocument();

    expect(clientMock.removeQOTDCollectorDeckDuplicates).toHaveBeenCalledWith(
      "guild-1",
      "default",
    );
    expect(clientMock.getQOTDSettings).toHaveBeenCalledTimes(2);
    expect(clientMock.getQOTDSummary).toHaveBeenCalledTimes(2);
    expect(clientMock.listQOTDQuestions).toHaveBeenCalledTimes(2);
  });
});

function QOTDContextHarness() {
  const { deckSummaries, questions, removeCollectorDeckDuplicates } = useQOTD();
  const [resultLabel, setResultLabel] = useState("");

  return (
    <div>
      <span>{deckSummaries[0]?.cards_remaining ?? 0} cards remaining</span>
      <span>{questions.length} questions loaded</span>
      <button
        type="button"
        onClick={async () => {
          const result = await removeCollectorDeckDuplicates("default");
          if (result != null) {
            setResultLabel(`${result.deleted_questions} deleted`);
          }
        }}
      >
        Remove duplicates
      </button>
      <span>{resultLabel}</span>
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

function createQuestions(count: number): QOTDQuestion[] {
  return Array.from({ length: count }, (_, index) => ({
    id: index + 1,
    display_id: index + 1,
    deck_id: "default",
    body: `Question ${index + 1}`,
    status: "ready",
    queue_position: index + 1,
    created_at: "2026-04-04T00:00:00Z",
    updated_at: "2026-04-04T00:00:00Z",
  }));
}