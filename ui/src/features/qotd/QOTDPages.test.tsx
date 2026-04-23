import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  AccessibleGuild,
  ControlApiClient,
  QOTDConfig,
  QOTDDeckSummary,
  QOTDQuestion,
  QOTDSummary,
} from "../../api/control";
import { appRoutes } from "../../app/routes";
import { QOTDLayout } from "./QOTDLayout";
import { QOTDCollectorPage } from "./QOTDCollectorPage";
import { QOTDQuestionsPage } from "./QOTDQuestionsPage";
import { QOTDSettingsPage } from "./QOTDSettingsPage";

const dashboardSessionMock: {
  authState: string;
  beginLogin: ReturnType<typeof vi.fn>;
  canEditSelectedGuild: boolean;
  client: Pick<
    ControlApiClient,
    | "downloadQOTDCollectorExport"
    | "getQOTDCollectorSummary"
    | "runQOTDCollector"
  >;
  selectedGuild: AccessibleGuild | null;
  selectedGuildID: string;
} = {
  authState: "signed_in",
  beginLogin: vi.fn(),
  canEditSelectedGuild: true,
  client: {
    downloadQOTDCollectorExport: vi.fn(),
    getQOTDCollectorSummary: vi.fn(),
    runQOTDCollector: vi.fn(),
  },
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
  createQuestions: vi.fn(),
  deleteQuestion: vi.fn(),
  publishNow: vi.fn(),
  refreshWorkspace: vi.fn(),
  reorderQuestions: vi.fn(),
  saveSettings: vi.fn(),
  setupForum: vi.fn(),
  selectDeck: vi.fn(),
  updateQuestion: vi.fn(),
};

const collectorSummaryMock = {
  total_questions: 1,
  recent_questions: [
    {
      id: 91,
      source_channel_id: "question-channel-1",
      source_message_id: "message-91",
      source_author_id: "bot-1",
      source_author_name: "QOTD Bot",
      source_created_at: "2026-04-13T15:00:00Z",
      embed_title: "Question Of The Day",
      question_text: "What is one habit you want to keep this month?",
      created_at: "2026-04-13T15:00:00Z",
      updated_at: "2026-04-13T15:00:00Z",
    },
  ],
};

const channelOptionsMock = {
  channels: [
    {
      id: "general-channel-1",
      name: "general",
      display_name: "#general",
      kind: "text",
      supports_message_route: true,
    },
    {
      id: "updates-channel-1",
      name: "updates",
      display_name: "#updates",
      kind: "text",
      supports_message_route: true,
    },
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
  const actual =
    await vi.importActual<typeof import("./QOTDContext")>("./QOTDContext");
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
    qotdMock.createQuestions.mockReset().mockResolvedValue(true);
    qotdMock.saveSettings.mockReset().mockImplementation(async (next) => next);
    qotdMock.setupForum.mockReset().mockResolvedValue(undefined);
    qotdMock.selectDeck.mockReset().mockResolvedValue(undefined);
    channelOptionsMock.refresh.mockReset();
    dashboardSessionMock.client.getQOTDCollectorSummary = vi
      .fn()
      .mockResolvedValue({
        status: "ok",
        guild_id: "guild-1",
        summary: collectorSummaryMock,
      });
    dashboardSessionMock.client.runQOTDCollector = vi.fn().mockResolvedValue({
      status: "ok",
      guild_id: "guild-1",
      result: {
        scanned_messages: 8,
        matched_messages: 3,
        new_questions: 2,
        total_questions: 3,
      },
      summary: {
        total_questions: 3,
        recent_questions: [
          {
            id: 99,
            source_channel_id: "question-channel-1",
            source_message_id: "message-99",
            source_author_id: "bot-1",
            source_author_name: "QOTD Bot",
            source_created_at: "2026-04-13T16:00:00Z",
            embed_title: "question!!",
            question_text: "What are you excited to try next?",
            created_at: "2026-04-13T16:00:00Z",
            updated_at: "2026-04-13T16:00:00Z",
          },
          ...collectorSummaryMock.recent_questions,
        ],
      },
    });
    dashboardSessionMock.client.downloadQOTDCollectorExport = vi
      .fn()
      .mockResolvedValue({
        filename: "qotd-collected-questions.txt",
        text: "First exported question\nSecond exported question\n",
      });
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

    expect(
      screen.getByRole("heading", { name: "QOTD", level: 1 }),
    ).toBeInTheDocument();
    expect(screen.getByText("Settings body")).toBeInTheDocument();
    expect(screen.queryByText("Current slot")).not.toBeInTheDocument();
    expect(screen.queryByText("Question")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Publish manual QOTD" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /refresh/i }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Workspace status")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Refreshing QOTD workspace..."),
    ).not.toBeInTheDocument();
  });

  it("renders the QOTD settings as grouped configuration sections", () => {
    render(
      <MemoryRouter>
        <QOTDSettingsPage />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole("heading", { name: "Workflow settings", level: 2 }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Staff roles", level: 2 }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(
        "Choose the forum and tags used by the daily publish flow.",
      ),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("1 roles")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /refresh tags/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Save changes" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Repair QOTD setup" }),
    ).toBeInTheDocument();
  });

  it("runs the automatic setup flow from the settings page", async () => {
    const user = userEvent.setup();

    const view = render(
      <MemoryRouter>
        <QOTDSettingsPage />
      </MemoryRouter>,
    );

    await user.click(
      within(view.container).getByRole("button", { name: "Repair QOTD setup" }),
    );

    await waitFor(() => {
      expect(qotdMock.setupForum).toHaveBeenCalledWith("default");
      expect(channelOptionsMock.refresh).toHaveBeenCalledTimes(1);
    });
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
            channel_id: "question-channel-1",
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
          channel_id: "answers-channel-1",
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

  it("allows deleting a deck with questions and explains the cascade", () => {
    qotdMock.settings = createQOTDSettings({
      decks: [
        {
          id: "default",
          name: "Default",
          enabled: true,
          channel_id: "question-channel-1",
        },
        {
          id: "deck-b",
          name: "Deck B",
          enabled: false,
          channel_id: "question-channel-1",
        },
      ],
    });
    qotdMock.summary = createQOTDSummary({
      settings: qotdMock.settings,
      decks: [
        createQOTDDeckSummaries({
          counts: {
            total: 0,
            draft: 0,
            ready: 0,
            reserved: 0,
            used: 0,
            disabled: 0,
          },
          cards_remaining: 0,
        })[0],
        {
          id: "deck-b",
          name: "Deck B",
          enabled: false,
          counts: {
            total: 2,
            draft: 1,
            ready: 1,
            reserved: 0,
            used: 0,
            disabled: 0,
          },
          cards_remaining: 2,
          is_active: false,
          can_publish: false,
        },
      ],
    });
    qotdMock.deckSummaries = qotdMock.summary.decks;

    render(
      <MemoryRouter>
        <QOTDSettingsPage />
      </MemoryRouter>,
    );

    expect(
      screen.getByText(
        "Deleting this deck also removes 2 questions from this bank.",
      ),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole("group", { name: "Deck B" })).getByRole(
        "button",
        {
          name: "Delete deck",
        },
      ),
    ).toBeEnabled();
  });

  it("renders the queue editor with question cards and local actions", () => {
    render(
      <MemoryRouter>
        <QOTDQuestionsPage />
      </MemoryRouter>,
    );

    expect(screen.getByText("Add a question")).toBeInTheDocument();
    expect(screen.getByText("Question order")).toBeInTheDocument();
    expect(
      screen.getByText("What is one thing you shipped this week?"),
    ).toBeInTheDocument();
    expect(screen.queryByText("1 ready")).not.toBeInTheDocument();
    expect(screen.queryByText("2 total")).not.toBeInTheDocument();
    expect(screen.queryByText("Status Ready")).not.toBeInTheDocument();
    expect(screen.getAllByRole("button", { name: "Move up" })).not.toHaveLength(
      0,
    );
  });

  it("imports questions from a text file into the selected deck", async () => {
    const user = userEvent.setup();

    const view = render(
      <MemoryRouter>
        <QOTDQuestionsPage />
      </MemoryRouter>,
    );

    const input = within(view.container).getByLabelText("Import from .txt");
    const fileContents =
      "First imported question\n\nSecond imported question  \r\nThird imported question";
    const file = new File([fileContents], "qotd.txt", {
      type: "text/plain",
    });
    Object.defineProperty(file, "text", {
      value: vi.fn().mockResolvedValue(fileContents),
    });

    await user.upload(input, file);

    await waitFor(() => {
      expect(
        within(view.container).getByText("3 questions ready to import."),
      ).toBeInTheDocument();
    });

    await user.click(
      within(view.container).getByRole("button", { name: "Import .txt" }),
    );

    expect(qotdMock.createQuestions).toHaveBeenCalledWith([
      {
        deck_id: "default",
        body: "First imported question",
        status: "ready",
      },
      {
        deck_id: "default",
        body: "Second imported question",
        status: "ready",
      },
      {
        deck_id: "default",
        body: "Third imported question",
        status: "ready",
      },
    ]);
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

    expect(
      screen.getByRole("button", { name: "Publish manual QOTD" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Questions body")).toBeInTheDocument();
  });

  it("saves collector settings without dropping the rest of the qotd config", async () => {
    const user = userEvent.setup();

    const view = render(
      <MemoryRouter>
        <QOTDCollectorPage />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(
        within(view.container).getByText("1 collected question stored"),
      ).toBeInTheDocument();
    });

    await user.selectOptions(
      within(view.container).getByLabelText("History channel"),
      "answers-channel-1",
    );
    await user.clear(
      within(view.container).getByLabelText("Allowed author IDs"),
    );
    await user.type(
      within(view.container).getByLabelText("Allowed author IDs"),
      "111111111111111111\n222222222222222222",
    );
    await user.clear(
      within(view.container).getByLabelText("Embed title patterns"),
    );
    await user.type(
      within(view.container).getByLabelText("Embed title patterns"),
      "Question Of The Day\nquestion!!",
    );
    await user.type(
      within(view.container).getByLabelText("Earliest message date"),
      "2026-01-01",
    );

    await user.click(
      within(view.container).getByRole("button", { name: "Save changes" }),
    );

    expect(qotdMock.saveSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        active_deck_id: "default",
        decks: expect.any(Array),
        collector: {
          source_channel_id: "answers-channel-1",
          author_ids: ["111111111111111111", "222222222222222222"],
          title_patterns: ["Question Of The Day", "question!!"],
          start_date: "2026-01-01",
        },
      }),
    );
  });

  it("normalizes collector draft values after save and clears the unsaved state", async () => {
    const user = userEvent.setup();
    qotdMock.saveSettings.mockImplementation(async (next) =>
      createQOTDSettings({
        ...next,
        collector: {
          source_channel_id: "answers-channel-1",
          author_ids: ["111111111111111111", "222222222222222222"],
          title_patterns: ["Question Of The Day", "question!!"],
          start_date: "2026-01-01",
        },
      }),
    );

    const view = render(
      <MemoryRouter>
        <QOTDCollectorPage />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(
        within(view.container).getByText("1 collected question stored"),
      ).toBeInTheDocument();
    });

    await user.selectOptions(
      within(view.container).getByLabelText("History channel"),
      "answers-channel-1",
    );
    await user.type(
      within(view.container).getByLabelText("Allowed author IDs"),
      "111111111111111111\n111111111111111111\n222222222222222222",
    );
    await user.type(
      within(view.container).getByLabelText("Embed title patterns"),
      "Question Of The Day\nquestion of the day\nquestion!!",
    );
    await user.type(
      within(view.container).getByLabelText("Earliest message date"),
      "2026-01-01",
    );

    await user.click(
      within(view.container).getByRole("button", { name: "Save changes" }),
    );

    await waitFor(() => {
      expect(
        within(view.container).queryByRole("button", { name: "Save changes" }),
      ).not.toBeInTheDocument();
    });

    expect(
      within(view.container).getByLabelText("Allowed author IDs"),
    ).toHaveValue("111111111111111111\n222222222222222222");
    expect(
      within(view.container).getByLabelText("Embed title patterns"),
    ).toHaveValue("Question Of The Day\nquestion!!");
  });

  it("runs collector scans and downloads the exported text file", async () => {
    const user = userEvent.setup();
    qotdMock.settings = createQOTDSettings({
      collector: {
        source_channel_id: "question-channel-1",
        author_ids: ["bot-1"],
        title_patterns: ["Question Of The Day", "question!!"],
      },
    });
    if (!("createObjectURL" in URL)) {
      Object.defineProperty(URL, "createObjectURL", {
        configurable: true,
        writable: true,
        value: () => "blob:collector",
      });
    }
    if (!("revokeObjectURL" in URL)) {
      Object.defineProperty(URL, "revokeObjectURL", {
        configurable: true,
        writable: true,
        value: () => undefined,
      });
    }
    const createObjectURL = vi
      .spyOn(URL, "createObjectURL")
      .mockReturnValue("blob:collector");
    const revokeObjectURL = vi
      .spyOn(URL, "revokeObjectURL")
      .mockImplementation(() => undefined);
    const anchorClick = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);

    const view = render(
      <MemoryRouter>
        <QOTDCollectorPage />
      </MemoryRouter>,
    );

    await waitFor(() => {
      expect(
        within(view.container).getByText("1 collected question stored"),
      ).toBeInTheDocument();
    });

    await user.click(
      within(view.container).getByRole("button", {
        name: "Collect questions now",
      }),
    );

    await waitFor(() => {
      expect(
        within(view.container).getByText(
          "Scanned 8 messages, matched 3 embeds, and stored 2 new questions. 3 total questions are ready for export.",
        ),
      ).toBeInTheDocument();
    });

    expect(dashboardSessionMock.client.runQOTDCollector).toHaveBeenCalledWith(
      "guild-1",
    );
    expect(
      within(view.container).getByText("3 collected questions stored"),
    ).toBeInTheDocument();

    await user.click(
      within(view.container).getByRole("button", { name: "Download .txt" }),
    );

    expect(
      dashboardSessionMock.client.downloadQOTDCollectorExport,
    ).toHaveBeenCalledWith("guild-1");
    expect(createObjectURL).toHaveBeenCalledTimes(1);
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:collector");

    createObjectURL.mockRestore();
    revokeObjectURL.mockRestore();
    anchorClick.mockRestore();
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
        channel_id: "question-channel-1",
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

function createQOTDSummary(overrides: Partial<QOTDSummary> = {}): QOTDSummary {
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
      deck_id: "default",
      deck_name: "Default",
      publish_mode: "scheduled",
      publish_date_utc: "2026-04-03T00:00:00Z",
      state: "published",
      question_text: "Yesterday's question",
      becomes_previous_at: "2026-04-04T00:00:00Z",
      answers_close_at: "2026-04-05T00:00:00Z",
    },
    ...overrides,
    settings,
  };
}
