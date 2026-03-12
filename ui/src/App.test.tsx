import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import App from "./App";

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
  const boardByGuild = {
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
  } as const;

  const fetchMock = vi.fn(async (input: RequestInfo | URL) => {
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

    if (url.includes("/partner-board") && !url.endsWith("/partner-board/sync")) {
      const match = url.match(/\/v1\/guilds\/([^/]+)\/partner-board$/);
      if (match) {
        const guildID = decodeURIComponent(match[1] ?? "");
        boardCalls.push(guildID);
        return jsonResponse({
          status: "ok",
          guild_id: guildID,
          partner_board: boardByGuild[guildID as keyof typeof boardByGuild],
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

  it("renders the shell and redirects the legacy control panel route to Partner Board entries", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);

    render(<App />);

    await screen.findByRole("heading", { name: "Partner Board", level: 1 });
    expect(screen.getByRole("link", { name: "Overview" })).toBeInTheDocument();
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

  it("keeps Entries, Layout, and Posting destination on separate routes", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/partner-board/layout");

    render(<App />);

    await screen.findByRole("heading", { name: "Board text" });
    expect(screen.queryByRole("heading", { name: "Manage entries" })).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("link", { name: "Posting destination" }));

    await screen.findByRole("heading", { name: "Where the board is posted" });
    expect(screen.queryByRole("heading", { name: "Board text" })).not.toBeInTheDocument();
  });

  it("uses a drawer for add and edit, inline confirmation for remove, and hides troubleshooting content by default", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/partner-board/entries");

    render(<App />);

    await screen.findByRole("heading", { name: "Manage entries" });

    const permissionsLabel = screen.getAllByText("Permissions granted")[0];
    expect(permissionsLabel).not.toBeVisible();

    await userEvent.click(screen.getByText("Troubleshooting"));
    expect(permissionsLabel).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "Add partner" }));
    expect(screen.getByLabelText("Add partner")).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    await userEvent.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByLabelText("Edit partner")).toBeVisible();

    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    await userEvent.click(screen.getByRole("button", { name: "Remove" }));
    expect(screen.getByRole("button", { name: "Confirm" })).toBeVisible();
  });
});
