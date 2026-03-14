import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import App from "./App";
import type { PartnerBoardConfig } from "./api/control";

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: {
      "Content-Type": "application/json",
    },
  });
}

function createFetchMock() {
  const boardCalls: string[] = [];
  const targetUpdates: Array<{
    guildID: string;
    payload: Record<string, unknown>;
  }> = [];
  const boardByGuild: Record<string, PartnerBoardConfig> = {
    "guild-1": {
      target: {
        type: "channel_message",
        message_id: "111111111111111111",
        channel_id: "222222222222222222",
      },
      template: {
        title: "Partner Board",
        intro: "Server one intro",
        section_header_template: "Section header",
        line_template: "Partner row",
        empty_state_text: "No partners yet",
      },
      partners: [
        {
          fandom: "Action",
          name: "Server One",
          link: "https://discord.gg/server-one",
        },
      ],
    },
    "guild-2": {
      target: {
        type: "webhook_message",
        message_id: "333333333333333333",
        webhook_url: "https://discord.com/api/webhooks/example",
      },
      template: {
        title: "Partner Board",
        intro: "Server two intro",
        section_header_template: "Section header",
        line_template: "Partner row",
        empty_state_text: "No partners yet",
      },
      partners: [
        {
          fandom: "Puzzle",
          name: "Server Two",
          link: "https://discord.gg/server-two",
        },
      ],
    },
  };

  const fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = typeof input === "string" ? input : input.toString();

    if (url.endsWith("/auth/me")) {
      return jsonResponse({
        status: "ok",
        user: {
          id: "user-1",
          username: "alice",
          global_name: "alice",
        },
        scopes: ["identify", "guilds"],
        csrf_token: "csrf-token",
        expires_at: "2099-01-01T00:00:00Z",
      });
    }

    if (url.endsWith("/auth/guilds/manageable")) {
      return jsonResponse({
        status: "ok",
        count: 2,
        guilds: [
          {
            id: "guild-1",
            name: "Server One",
            owner: true,
            permissions: 8,
          },
          {
            id: "guild-2",
            name: "Server Two",
            owner: false,
            permissions: 32,
          },
        ],
      });
    }

    if (url.includes("/partner-board/target") && init?.method === "PUT") {
      const match = url.match(/\/v1\/guilds\/([^/]+)\/partner-board\/target$/);
      if (match) {
        const guildID = decodeURIComponent(match[1] ?? "");
        const payload = JSON.parse(String(init.body)) as Record<string, unknown>;
        targetUpdates.push({ guildID, payload });
        const nextBoard = boardByGuild[guildID];
        nextBoard.target = {
          ...nextBoard.target,
          ...(payload as PartnerBoardConfig["target"]),
        };

        if (payload.type === "channel_message") {
          delete nextBoard.target?.webhook_url;
        }
        if (payload.type === "webhook_message") {
          delete nextBoard.target?.channel_id;
        }

        return jsonResponse({
          status: "ok",
          guild_id: guildID,
          target: nextBoard.target,
        });
      }
    }

    if (url.includes("/partner-board") && !url.endsWith("/partner-board/sync")) {
      const match = url.match(/\/v1\/guilds\/([^/]+)\/partner-board$/);
      if (match) {
        const guildID = decodeURIComponent(match[1] ?? "");
        boardCalls.push(guildID);
        return jsonResponse({
          status: "ok",
          guild_id: guildID,
          partner_board: boardByGuild[guildID],
        });
      }
    }

    if (url.endsWith("/partner-board/sync")) {
      return jsonResponse({
        status: "ok",
        guild_id: "guild-1",
        synced: true,
      });
    }

    return new Response("not found", { status: 404 });
  });

  return {
    boardCalls,
    fetchMock,
    targetUpdates,
  };
}

describe("dashboard routing and workspace", () => {
  beforeEach(() => {
    window.history.replaceState({}, "", "/dashboard/control-panel");
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
    window.history.replaceState({}, "", "/");
  });

  it("renders the lean shell, preserves the legacy control-panel redirect, and keeps only real primary nav items", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);

    await screen.findByRole("heading", { name: "Partner Board", level: 1 });
    expect(screen.getByRole("link", { name: "Overview" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Partner Board" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Settings" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Moderation" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Automations" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Activity Log" })).not.toBeInTheDocument();
    expect(
      await screen.findByRole("heading", { name: "Manage entries" }),
    ).toBeInTheDocument();
    expect(window.location.pathname).toBe("/dashboard/partner-board/entries");
  });

  it("auto-loads Partner Board data again when the selected server changes", async () => {
    const { boardCalls, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/partner-board/entries");

    render(<App />);

    await screen.findByRole("heading", { name: "Partner Board", level: 1 });
    const serverSelect = await screen.findByLabelText("Server");

    await waitFor(() => {
      expect(boardCalls).toContain("guild-1");
    });

    await userEvent.selectOptions(serverSelect, "guild-2");

    await waitFor(() => {
      expect(boardCalls).toContain("guild-2");
    });

    await screen.findByRole("cell", { name: "Server Two" });
  });

  it.each([
    "/dashboard/moderation",
    "/dashboard/automations",
    "/dashboard/activity",
  ])("redirects %s to the overview roadmap instead of a placeholder page", async (path) => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", path);

    render(<App />);

    await screen.findByRole("heading", { name: "Overview", level: 1 });
    expect(window.location.pathname).toBe("/dashboard/overview");
    expect(window.location.hash).toBe("#roadmap");
    expect(screen.getByText("Planned areas")).toBeInTheDocument();
    expect(screen.getByText("Moderation")).toBeInTheDocument();
    expect(screen.getByText("Automations")).toBeInTheDocument();
    expect(screen.getByText("Global activity")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Moderation" })).not.toBeInTheDocument();
  });

  it("keeps Entries, Layout, and Destination on separate routes and removes the placeholder Activity tab", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/partner-board/layout");

    render(<App />);

    await screen.findByRole("heading", { name: "Board text" });
    expect(screen.queryByRole("heading", { name: "Manage entries" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Activity" })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("link", { name: "Destination" }));

    await screen.findByRole("heading", { name: "Set where the board is published" });
    expect(screen.queryByRole("heading", { name: "Board text" })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Board message ID")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Channel ID")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Webhook URL")).not.toBeInTheDocument();
  });

  it("uses a drawer for add and edit, and inline confirmation for remove on Partner Board entries", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/partner-board/entries");

    render(<App />);

    await screen.findByRole("heading", { name: "Manage entries" });

    await userEvent.click(screen.getByRole("button", { name: "Add partner" }));
    expect(screen.getByLabelText("Add partner")).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    await userEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByLabelText("Edit partner")).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await userEvent.click(screen.getByRole("button", { name: "Remove" }));
    expect(screen.getByRole("button", { name: "Confirm" })).toBeVisible();
  });

  it("shows a real Overview feature card and keeps roadmap items informational only", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/overview");

    render(<App />);

    await screen.findByRole("heading", { name: "Overview", level: 1 });
    expect(screen.getByRole("heading", { name: "Partner Board", level: 2 })).toBeInTheDocument();
    expect(screen.getByText("Feature status")).toBeInTheDocument();
    expect(screen.getByText("Partner entries")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Finish destination" })).toBeInTheDocument();
    expect(screen.getByText("Planned areas")).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Global activity" })).not.toBeInTheDocument();
  });

  it("hands off destination setup to Settings diagnostics with the requested posting method preselected", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/partner-board/delivery");

    render(<App />);

    await screen.findByRole("heading", { name: "Set where the board is published" });

    await userEvent.selectOptions(
      screen.getByLabelText("Preferred posting method"),
      "webhook_message",
    );
    await userEvent.click(
      screen.getByRole("link", { name: "Finish destination in Settings" }),
    );

    await screen.findByRole("heading", { name: "Settings", level: 1 });
    expect(window.location.pathname).toBe("/dashboard/settings");
    expect(window.location.hash).toBe("#diagnostics");
    expect(screen.getByText("Granted OAuth scopes")).toBeVisible();
    expect(screen.getByLabelText("Posting method")).toHaveValue("webhook_message");
    expect(screen.getByLabelText("Board message ID")).toBeVisible();
  });

  it("keeps raw technical details hidden until Diagnostics is opened and still saves the advanced destination editor", async () => {
    const { fetchMock, targetUpdates } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/settings");

    render(<App />);

    await screen.findByRole("heading", { name: "Settings", level: 1 });

    expect(screen.getByText("Granted OAuth scopes")).not.toBeVisible();
    expect(screen.getByText("Board message ID")).not.toBeVisible();

    await userEvent.click(screen.getByText("Diagnostics", { selector: "summary" }));

    expect(screen.getByText("Granted OAuth scopes")).toBeVisible();
    await userEvent.selectOptions(screen.getByLabelText("Posting method"), "webhook_message");
    await userEvent.clear(screen.getByLabelText("Board message ID"));
    await userEvent.type(screen.getByLabelText("Board message ID"), "999999999999999999");
    await userEvent.clear(screen.getByLabelText("Webhook URL"));
    await userEvent.type(
      screen.getByLabelText("Webhook URL"),
      "https://discord.com/api/webhooks/new-target",
    );
    await userEvent.click(screen.getByRole("button", { name: "Save destination" }));

    await waitFor(() => {
      expect(targetUpdates).toEqual([
        {
          guildID: "guild-1",
          payload: {
            type: "webhook_message",
            message_id: "999999999999999999",
            webhook_url: "https://discord.com/api/webhooks/new-target",
          },
        },
      ]);
    });

    expect(screen.getByText("Partner Board destination updated.")).toBeInTheDocument();
  });
});
