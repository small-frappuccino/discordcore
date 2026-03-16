import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import App from "./App";
import type { FeatureRecord, PartnerBoardConfig } from "./api/control";

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
  const featureCalls: string[] = [];
  const featureUpdates: Array<{
    guildID: string;
    featureID: string;
    payload: Record<string, unknown>;
  }> = [];
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
  const featuresByGuild: Record<string, FeatureRecord[]> = {
    "guild-1": [
      {
        id: "services.monitoring",
        category: "services",
        label: "Monitoring",
        description: "Core monitoring",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
      },
      {
        id: "services.commands",
        category: "services",
        label: "Commands",
        description: "Commands",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
      },
      {
        id: "services.admin_commands",
        category: "services",
        label: "Admin commands",
        description: "Admin commands",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
      },
      {
        id: "services.automod",
        category: "services",
        label: "Automod service",
        description: "Automod service",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "blocked",
        blockers: [
          {
            code: "missing_rules",
            message: "Automod needs rules before it can be relied on.",
          },
        ],
      },
      {
        id: "logging.avatar_logging",
        category: "logging",
        label: "Avatar logging",
        description: "Avatar changes",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
        details: {
          requires_channel: true,
          channel_id: "user-log-channel",
          validate_channel_permissions: true,
          runtime_toggle_path: "disable_user_logs",
        },
        editable_fields: ["enabled", "channel_id"],
      },
      {
        id: "logging.member_join",
        category: "logging",
        label: "Member join logging",
        description: "Member joins",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "blocked",
        blockers: [
          {
            code: "missing_channel",
            message: "Choose a channel for member join logs.",
          },
        ],
        details: {
          requires_channel: true,
          validate_channel_permissions: true,
          runtime_toggle_path: "disable_entry_exit_logs",
        },
        editable_fields: ["enabled", "channel_id"],
      },
      {
        id: "presence_watch.user",
        category: "presence_watch",
        label: "Presence watch (user)",
        description: "Presence watch",
        scope: "guild",
        supports_guild_override: true,
        override_state: "disabled",
        effective_enabled: false,
        effective_source: "guild",
        readiness: "disabled",
      },
      {
        id: "auto_role_assignment",
        category: "roles",
        label: "Auto role assignment",
        description: "Auto role",
        scope: "guild",
        supports_guild_override: true,
        override_state: "disabled",
        effective_enabled: false,
        effective_source: "guild",
        readiness: "disabled",
      },
      {
        id: "maintenance.db_cleanup",
        category: "maintenance",
        label: "Database cleanup",
        description: "Cleanup",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
      },
      {
        id: "stats_channels",
        category: "stats",
        label: "Stats channels",
        description: "Stats",
        scope: "guild",
        supports_guild_override: true,
        override_state: "disabled",
        effective_enabled: false,
        effective_source: "guild",
        readiness: "disabled",
      },
    ],
    "guild-2": [
      {
        id: "services.monitoring",
        category: "services",
        label: "Monitoring",
        description: "Core monitoring",
        scope: "guild",
        supports_guild_override: true,
        override_state: "enabled",
        effective_enabled: true,
        effective_source: "guild",
        readiness: "ready",
      },
    ],
  };

  function evaluateFeatureState(feature: FeatureRecord) {
    const requiresChannel =
      feature.details !== undefined &&
      feature.details !== null &&
      feature.details.requires_channel === true;
    const channelId =
      feature.details !== undefined &&
      feature.details !== null &&
      typeof feature.details.channel_id === "string"
        ? feature.details.channel_id.trim()
        : "";

    if (!feature.effective_enabled) {
      feature.readiness = "disabled";
      feature.blockers = [];
      return;
    }

    if (requiresChannel && channelId === "") {
      feature.readiness = "blocked";
      feature.blockers = [
        {
          code: "missing_channel",
          message: "Choose a channel for this log route.",
          field: "channel_id",
        },
      ];
      return;
    }

    if (feature.id === "services.automod") {
      feature.readiness = "blocked";
      feature.blockers = [
        {
          code: "missing_rules",
          message: "Automod needs rules before it can be relied on.",
        },
      ];
      return;
    }

    feature.readiness = "ready";
    feature.blockers = [];
  }

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

    if (url.includes("/features/") && init?.method === "PATCH") {
      const match = url.match(/\/v1\/guilds\/([^/]+)\/features\/([^/?]+)/);
      if (match) {
        const guildID = decodeURIComponent(match[1] ?? "");
        const featureID = decodeURIComponent(match[2] ?? "");
        const payload = JSON.parse(String(init.body)) as Record<string, unknown>;
        featureUpdates.push({
          guildID,
          featureID,
          payload,
        });

        const feature = featuresByGuild[guildID]?.find(
          (item) => item.id === featureID,
        );
        if (feature === undefined) {
          return new Response("not found", { status: 404 });
        }

        if (Object.prototype.hasOwnProperty.call(payload, "enabled")) {
          const enabled = payload.enabled;
          if (enabled === null) {
            feature.override_state = "inherit";
            feature.effective_enabled = false;
            feature.effective_source = "global";
          } else {
            const nextEnabled = Boolean(enabled);
            feature.override_state = nextEnabled ? "enabled" : "disabled";
            feature.effective_enabled = nextEnabled;
            feature.effective_source = "guild";
          }
        }

        if (Object.prototype.hasOwnProperty.call(payload, "channel_id")) {
          feature.details = {
            ...(feature.details ?? {}),
            channel_id: String(payload.channel_id ?? ""),
          };
        }

        evaluateFeatureState(feature);

        return jsonResponse({
          status: "ok",
          guild_id: guildID,
          feature: {
            ...feature,
          },
        });
      }
    }

    if (url.includes("/features") && !url.includes("/catalog")) {
      const match = url.match(/\/v1\/guilds\/([^/]+)\/features$/);
      if (match) {
        const guildID = decodeURIComponent(match[1] ?? "");
        featureCalls.push(guildID);
        return jsonResponse({
          status: "ok",
          workspace: {
            scope: "guild",
            guild_id: guildID,
            features: featuresByGuild[guildID] ?? [],
          },
        });
      }
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
    featureCalls,
    fetchMock,
    featureUpdates,
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

    await screen.findByRole("heading", { name: "Home", level: 1 });
    expect(screen.getByRole("link", { name: "Home" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Partner Board" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Commands" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Moderation" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Logging" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Roles & Members" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Maintenance" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Stats" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Settings" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Automations" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "Activity Log" })).not.toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: "Commands", level: 2 })).toBeInTheDocument();
    expect(window.location.pathname).toBe("/dashboard/home");
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

  it("opens Moderation as a real category workspace instead of redirecting to Home", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/moderation");

    render(<App />);

    await screen.findByRole("heading", { name: "Moderation", level: 1 });
    expect(window.location.pathname).toBe("/dashboard/moderation");
    expect(window.location.hash).toBe("");
    expect(
      await screen.findByRole("button", { name: "Disable Automod service" }),
    ).toBeInTheDocument();
  });

  it.each([
    "/dashboard/automations",
    "/dashboard/activity",
  ])("redirects %s to the planned Home section instead of a placeholder page", async (path) => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", path);

    render(<App />);

    await screen.findByRole("heading", { name: "Home", level: 1 });
    expect(window.location.pathname).toBe("/dashboard/home");
    expect(window.location.hash).toBe("#planned");
    expect(screen.getByText("Tickets")).toBeInTheDocument();
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

  it("shows Home as the operational landing page with area blocks and planned modules", async () => {
    const { fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/home");

    render(<App />);

    await screen.findByRole("heading", { name: "Home", level: 1 });
    expect(screen.getByRole("heading", { name: "Partner Board", level: 2 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Commands", level: 2 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Moderation", level: 2 })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Settings", level: 2 })).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: "Open Partner Board" }).length).toBeGreaterThan(0);
    expect(screen.getByRole("link", { name: "Open Commands" })).toBeInTheDocument();
    expect(screen.getByText("Tickets")).toBeInTheDocument();
  });

  it("opens a generic category workspace from the sidebar and updates feature state inline", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/home");

    render(<App />);

    await screen.findByRole("heading", { name: "Home", level: 1 });
    await userEvent.click(screen.getByRole("link", { name: "Commands" }));

    await screen.findByRole("heading", { name: "Commands", level: 1 });
    expect(window.location.pathname).toBe("/dashboard/commands");
    expect(screen.getByText("Monitoring")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Disable Commands" })).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Disable Commands" }));

    await waitFor(() => {
      expect(featureUpdates).toEqual([
        {
          guildID: "guild-1",
          featureID: "services.commands",
          payload: {
            enabled: false,
          },
        },
      ]);
    });

    await screen.findByRole("button", { name: "Enable Commands" });
  });

  it("opens the dedicated logging workspace and saves a destination through the drawer", async () => {
    const { featureUpdates, fetchMock } = createFetchMock();
    vi.stubGlobal("fetch", fetchMock);
    window.history.replaceState({}, "", "/dashboard/logging");

    render(<App />);

    await screen.findByRole("heading", { name: "Logging", level: 1 });
    expect(await screen.findByText("user-log-channel")).toBeInTheDocument();
    expect(await screen.findByText("Not configured")).toBeInTheDocument();

    const configureButtons = await screen.findAllByRole("button", {
      name: "Configure",
    });
    await userEvent.click(
      configureButtons[0]!,
    );

    expect(
      screen.getByRole("dialog", { name: "Configure Avatar logging" }),
    ).toBeVisible();
    expect(screen.getByLabelText("Destination channel")).toHaveValue(
      "user-log-channel",
    );
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    await userEvent.click(
      screen.getAllByRole("button", { name: "Configure" })[1]!,
    );
    expect(
      screen.getByRole("dialog", { name: "Configure Member join logging" }),
    ).toBeVisible();
    expect(screen.getByLabelText("Destination channel")).toHaveValue("");

    await userEvent.type(
      screen.getByLabelText("Destination channel"),
      "join-channel",
    );
    await userEvent.click(
      screen.getByRole("button", { name: "Save destination" }),
    );

    await waitFor(() => {
      expect(featureUpdates).toEqual([
        {
          guildID: "guild-1",
          featureID: "logging.member_join",
          payload: {
            channel_id: "join-channel",
          },
        },
      ]);
    });

    expect(
      screen.queryByRole("dialog", { name: "Configure Member join logging" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("join-channel")).toBeInTheDocument();
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
