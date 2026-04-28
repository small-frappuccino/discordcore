import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type {
  AccessibleGuild,
  ControlApiClient,
  QOTDConfig,
  QOTDDeckSummary,
  QOTDSummary,
} from "../../api/control";
import { appRoutes } from "../../app/routes";
import { QOTDLayout } from "./QOTDLayout";
import { QOTDSettingsPage } from "./QOTDSettingsPage";

const dashboardSessionMock: {
  authState: string;
  beginLogin: ReturnType<typeof vi.fn>;
  canEditSelectedGuild: boolean;
  client: Pick<ControlApiClient, never>;
  selectedGuild: AccessibleGuild | null;
  selectedGuildID: string;
} = {
  authState: "signed_in",
  beginLogin: vi.fn(),
  canEditSelectedGuild: true,
  client: {},
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
  settings: createQOTDSettings(),
  workspaceState: "ready",
  refreshWorkspace: vi.fn(),
  saveSettings: vi.fn(),
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
    qotdMock.settings = createQOTDSettings();
    qotdMock.workspaceState = "ready";
    qotdMock.saveSettings.mockReset().mockImplementation(async (next) => next);
    channelOptionsMock.refresh.mockReset();
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
      screen.queryByRole("navigation", { name: "QOTD sections" }),
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
      screen.getByRole("heading", { name: "Manage decks", level: 2 }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Settings")).not.toBeInTheDocument();
    expect(screen.queryByText("Decks")).not.toBeInTheDocument();
    expect(
      screen.queryByText(/create the qotd text channel manually in discord/i),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "Staff roles", level: 2 }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("1 roles")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /refresh tags/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Save changes" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /qotd setup/i }),
    ).not.toBeInTheDocument();
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

  it("preserves legacy hidden qotd settings when saving the settings page", async () => {
    const user = userEvent.setup();
    qotdMock.settings = createQOTDSettings({
      verified_role_id: "987654321098765432",
    });

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
        verified_role_id: "987654321098765432",
        active_deck_id: "default",
        decks: expect.arrayContaining([
          expect.objectContaining({
            id: "default",
            enabled: false,
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
    const nextSummary = createQOTDSummary({
      settings: qotdMock.settings,
    });
    qotdMock.deckSummaries = nextSummary.decks;
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
    const nextSummary = createQOTDSummary({
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
    qotdMock.deckSummaries = nextSummary.decks;

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
});

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
    ...overrides,
    settings,
  };
}
