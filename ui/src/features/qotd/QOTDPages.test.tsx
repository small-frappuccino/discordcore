import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  AccessibleGuild,
  QOTDConfig,
  QOTDDeckSummary,
  QOTDQuestion,
  QOTDSummary,
} from "../../api/control";
import { appRoutes } from "../../app/routes";
import { QOTDLayout } from "./QOTDLayout";
import { QOTDQuestionsPage } from "./QOTDQuestionsPage";
import { QOTDSettingsPage } from "./QOTDSettingsPage";

const dashboardSessionMock: {
  authState: string;
  beginLogin: ReturnType<typeof vi.fn>;
  canEditSelectedGuild: boolean;
  selectedGuild: AccessibleGuild | null;
  selectedGuildID: string;
} = {
  authState: "signed_in",
  beginLogin: vi.fn(),
  canEditSelectedGuild: true,
  selectedGuild: {
    id: "guild-1",
    name: "Test Guild",
    icon: undefined,
    owner: true,
    permissions: 0,
    access_level: "write",
  },
  selectedGuildID: "guild-1",
};

const qotdMock = {
  busyLabel: "",
  deckSummaries: createQOTDDeckSummaries(),
  hasLoadedAttempt: true,
  loading: false,
  notice: null,
  questions: [
    createQuestion({
      id: 1,
      deck_id: "default",
      queue_position: 1,
      body: "What is one thing you shipped this week?",
      status: "ready",
    }),
    createQuestion({
      id: 2,
      deck_id: "default",
      queue_position: 2,
      body: "What UI detail still feels off?",
      status: "draft",
    }),
  ] as QOTDQuestion[],
  selectedDeckID: "default",
  settings: createQOTDSettings(),
  summary: createQOTDSummary(),
  workspaceState: "ready",
  clearNotice: vi.fn(),
  createQuestion: vi.fn(),
  deleteQuestion: vi.fn(),
  publishNow: vi.fn(),
  refreshWorkspace: vi.fn(),
  reorderQuestions: vi.fn(),
  saveSettings: vi.fn(),
  selectDeck: vi.fn(),
  updateQuestion: vi.fn(),
};

const channelOptionsMock = {
  channels: [
    {
      id: "question-channel-1",
      name: "qotd",
      display_name: "#qotd",
      kind: "text",
      supports_message_route: true,
    },
    {
      id: "answers-channel-1",
      name: "qotd-answers",
      display_name: "#qotd-answers",
      kind: "text",
      supports_message_route: true,
    },
  ],
  loading: false,
  notice: null,
  refresh: vi.fn(),
};

vi.mock("../../context/DashboardSessionContext", () => ({
  useDashboardSession: () => dashboardSessionMock,
}));

vi.mock("./QOTDContext", async () => {
  const actual = await vi.importActual<typeof import("./QOTDContext")>("./QOTDContext");
  return {
    ...actual,
    useQOTD: () => qotdMock,
  };
});

vi.mock("../features/useGuildChannelOptions", () => ({
  useGuildChannelOptions: () => channelOptionsMock,
}));

describe("QOTD UI", () => {
  beforeEach(() => {
    qotdMock.busyLabel = "";
    qotdMock.deckSummaries = createQOTDDeckSummaries();
    qotdMock.loading = false;
    qotdMock.notice = null;
    qotdMock.selectedDeckID = "default";
    qotdMock.settings = createQOTDSettings();
    qotdMock.summary = createQOTDSummary();
    qotdMock.workspaceState = "ready";
    qotdMock.saveSettings.mockReset().mockResolvedValue(undefined);
    qotdMock.selectDeck.mockReset().mockResolvedValue(undefined);
  });

  it("keeps refresh actions out of the ready settings shell", () => {
    qotdMock.loading = true;
    qotdMock.busyLabel = "Refreshing QOTD workspace...";

    render(
      <MemoryRouter initialEntries={[appRoutes.qotdSettings("guild-1")]}>
        <Routes>
          <Route path="/manage/:guildId/qotd" element={<QOTDLayout />}>
            <Route path="settings" element={<div>Settings body</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByRole("heading", { name: "QOTD", level: 1 })).toBeInTheDocument();
    expect(screen.getByText("Settings body")).toBeInTheDocument();
    expect(screen.queryByText("Current slot")).not.toBeInTheDocument();
    expect(screen.queryByText("Queue")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Publish manual QOTD" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /refresh/i })).not.toBeInTheDocument();
    expect(screen.queryByText("Workspace status")).not.toBeInTheDocument();
    expect(screen.queryByText("Refreshing QOTD workspace...")).not.toBeInTheDocument();
  });

  it("renders the QOTD settings as grouped configuration sections", () => {
    render(
      <MemoryRouter>
        <QOTDSettingsPage />
      </MemoryRouter>,
    );

    expect(screen.getByRole("heading", { name: "Workflow settings", level: 2 })).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Staff roles", level: 2 })).not.toBeInTheDocument();
    expect(screen.queryByText("Choose the forum and tags used by the daily publish flow.")).not.toBeInTheDocument();
    expect(screen.queryByText("1 roles")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /refresh tags/i })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Save changes" })).not.toBeInTheDocument();
  });

  it("shows the unsaved changes bar and resets the local draft", async () => {
    const user = userEvent.setup();

    const view = render(
      <MemoryRouter>
        <QOTDSettingsPage />
      </MemoryRouter>,
    );

    const enabledToggle = within(view.container).getByRole("checkbox", {
      name: /Enable Default/,
    });

    expect(enabledToggle).toBeChecked();

    await user.click(enabledToggle);

    expect(enabledToggle).not.toBeChecked();
    expect(
      within(view.container).getByRole("button", { name: "Reset" }),
    ).toBeInTheDocument();
    expect(
      within(view.container).getByRole("button", { name: "Save changes" }),
    ).toBeInTheDocument();

    await user.click(
      within(view.container).getByRole("button", { name: "Reset" }),
    );

    expect(enabledToggle).toBeChecked();
    expect(
      within(view.container).queryByRole("button", { name: "Save changes" }),
    ).not.toBeInTheDocument();
  });

  it("saves the local settings draft without forcing a workspace reload", async () => {
    const user = userEvent.setup();

    const view = render(
      <MemoryRouter>
        <QOTDSettingsPage />
      </MemoryRouter>,
    );

    await user.click(
      within(view.container).getByRole("checkbox", {
        name: /Enable Default/,
      }),
    );
    await user.click(
      within(view.container).getByRole("button", { name: "Save changes" }),
    );

    expect(qotdMock.saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        active_deck_id: "default",
        decks: expect.arrayContaining([
          expect.objectContaining({
            id: "default",
            name: "Default",
            enabled: false,
            question_channel_id: "question-channel-1",
            response_channel_id: "answers-channel-1",
          }),
        ]),
      }),
    );
  });

  it("keeps a dirty settings draft when a newer workspace snapshot arrives", async () => {
    const user = userEvent.setup();
    const view = render(
      <MemoryRouter>
        <QOTDSettingsPage />
      </MemoryRouter>,
    );

    await user.click(
      within(view.container).getByRole("checkbox", {
        name: /Enable Default/,
      }),
    );

    qotdMock.settings = createQOTDSettings({
      decks: [
        {
          id: "default",
          name: "Default",
          enabled: true,
          question_channel_id: "question-channel-2",
          response_channel_id: "answers-channel-1",
        },
      ],
    });
    qotdMock.summary = createQOTDSummary({
      settings: qotdMock.settings,
    });
    qotdMock.deckSummaries = qotdMock.summary.decks;
    view.rerender(
      <MemoryRouter>
        <QOTDSettingsPage />
      </MemoryRouter>,
    );

    expect(
      within(view.container).getByRole("checkbox", {
        name: /Enable Default/,
      }),
    ).not.toBeChecked();
    expect(
      within(view.container).getByRole("button", { name: "Save changes" }),
    ).toBeInTheDocument();
  });

  it("renders the queue editor with question cards and local actions", () => {
    render(
      <MemoryRouter>
        <QOTDQuestionsPage />
      </MemoryRouter>,
    );

    expect(screen.getByText("Add a question")).toBeInTheDocument();
    expect(screen.getByText("Question order")).toBeInTheDocument();
    expect(screen.getByText("What is one thing you shipped this week?")).toBeInTheDocument();
    expect(screen.queryByText("1 ready")).not.toBeInTheDocument();
    expect(screen.queryByText("2 total")).not.toBeInTheDocument();
    expect(screen.queryByText("Status Ready")).not.toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Move up" })).not.toHaveLength(0);
  });

  it("keeps manual publish on the questions route only", () => {
    render(
      <MemoryRouter initialEntries={[appRoutes.qotdQuestions("guild-1")]}>
        <Routes>
          <Route path="/manage/:guildId/qotd" element={<QOTDLayout />}>
            <Route path="questions" element={<div>Questions body</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    );

    expect(screen.getByRole("button", { name: "Publish manual QOTD" })).toBeInTheDocument();
    expect(screen.getByText("Questions body")).toBeInTheDocument();
  });
});

function createQuestion(overrides: Partial<QOTDQuestion>): QOTDQuestion {
  return {
    id: 1,
    deck_id: "default",
    body: "Question",
    status: "ready",
    queue_position: 1,
    created_at: "2026-04-04T00:00:00Z",
    updated_at: "2026-04-04T00:00:00Z",
    ...overrides,
  };
}

function createQOTDSettings(overrides: Partial<QOTDConfig> = {}): QOTDConfig {
  return {
    active_deck_id: "default",
    decks: [
      {
        id: "default",
        name: "Default",
        enabled: true,
        question_channel_id: "question-channel-1",
        response_channel_id: "answers-channel-1",
      },
    ],
    ...overrides,
  };
}

function createQOTDDeckSummaries(
  overrides: Partial<QOTDDeckSummary> = {},
): QOTDDeckSummary[] {
  return [
    {
      id: "default",
      name: "Default",
      enabled: true,
      counts: {
        total: 2,
        draft: 1,
        ready: 1,
        reserved: 0,
        used: 0,
        disabled: 0,
      },
      cards_remaining: 2,
      is_active: true,
      can_publish: true,
      ...overrides,
    },
  ];
}

function createQOTDSummary(
  overrides: Partial<QOTDSummary> = {},
): QOTDSummary {
  const settings = overrides.settings ?? createQOTDSettings();
  const decks = overrides.decks ?? createQOTDDeckSummaries();
  return {
    counts: {
      total: 2,
      draft: 1,
      ready: 1,
      reserved: 0,
      used: 0,
      disabled: 0,
    },
    current_publish_date_utc: "2026-04-04T00:00:00Z",
    decks,
    published_for_current_slot: false,
    previous_post: {
      id: 8,
      deck_id: "default",
      deck_name: "Default",
      question_id: 22,
      publish_mode: "scheduled",
      publish_date_utc: "2026-04-03T00:00:00Z",
      state: "published",
      question_channel_id: "question-channel-1",
      question_text_snapshot: "Yesterday's question",
      is_pinned: false,
      grace_until: "2026-04-04T00:00:00Z",
      archive_at: "2026-04-05T00:00:00Z",
    },
    ...overrides,
    settings,
  };
}
