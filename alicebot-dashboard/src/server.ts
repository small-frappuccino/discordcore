import { Elysia } from "elysia";
import cors from "@elysiajs/cors";
import { resolve } from "node:path";
import { stat } from "node:fs/promises";
import { AliceBotAdapter } from "./adapter/alicebot";
import { LogStore } from "./logs";
import { loadSettings, saveSettings, validateSettingsPayload } from "./settings";
import type { BotStatus, ProcessStatus, Settings } from "./types";

const appHost = "127.0.0.1";
const appPort = 3130;
const allowedOrigin = `http://${appHost}:${appPort}`;
const adminToken = process.env.ADMIN_TOKEN ?? "change-me";

const logs = new LogStore(5000);
let settings = await loadSettings();

const wsClients = new Set<any>();

const broadcast = (payload: unknown) => {
  const message = JSON.stringify(payload);
  for (const client of wsClients) {
    try {
      client.send(message);
    } catch {
      // ignore
    }
  }
};

const botStatus: BotStatus = {
  connected: false,
  username: "alicebot",
  guildCount: settings.guilds.length,
  uptimeSeconds: 0,
};

const adapter = new AliceBotAdapter(
  () => settings.executablePath,
  (log) => {
    logs.add(log);
    broadcast({ type: "log", data: log });
  },
  (status) => {
    broadcast({ type: "status", data: status });
  }
);

const ensureAdmin = (request: Request): Response | null => {
  const token = request.headers.get("X-Admin-Token");
  if (!token || token !== adminToken) {
    return new Response("Unauthorized", { status: 401 });
  }
  return null;
};

const getProcessStatus = (): ProcessStatus => adapter.getStatus();

const indexFile = resolve("./client/dist/index.html");
const staticDir = resolve("./client/dist/static");

const app = new Elysia()
  .use(
    cors({
      origin: allowedOrigin,
      methods: ["GET", "POST", "PUT"],
      allowedHeaders: ["Content-Type", "X-Admin-Token"],
    })
  )
  .get("/api/health", () => ({ status: "ok" }))
  .get("/api/status", () => ({
    bot: {
      ...botStatus,
      guildCount: settings.guilds.length,
    },
    process: getProcessStatus(),
  }))
  .get("/api/guilds", () => settings.guilds)
  .get("/api/services", () => [
    { id: "monitoring", name: "Monitoring", status: "healthy" },
    { id: "automod", name: "Automod", status: "degraded" },
    { id: "commands", name: "Commands", status: "healthy" },
  ])
  .get("/api/settings", () => settings)
  .put("/api/settings", async ({ request }) => {
    const auth = ensureAdmin(request);
    if (auth) return auth;

    const payload = await request.json();
    const validation = validateSettingsPayload(payload);
    if (!validation.ok) {
      return new Response(validation.error, { status: 400 });
    }

    settings = validation.settings;
    await saveSettings(settings);
    return settings;
  })
  .get("/api/logs", ({ query }) => {
    const limit = Number(query.limit ?? "200");
    return logs.list(Number.isNaN(limit) ? 200 : limit);
  })
  .post("/api/process/start", ({ request }) => {
    const auth = ensureAdmin(request);
    if (auth) return auth;
    return adapter.start();
  })
  .post("/api/process/stop", ({ request }) => {
    const auth = ensureAdmin(request);
    if (auth) return auth;
    return adapter.stop();
  })
  .post("/api/process/restart", ({ request }) => {
    const auth = ensureAdmin(request);
    if (auth) return auth;
    return adapter.restart();
  })
  .post("/api/process/validate-path", async ({ request }) => {
    const auth = ensureAdmin(request);
    if (auth) return auth;
    const body = (await request.json().catch(() => null)) as
      | { path?: string }
      | null;
    const path = body?.path;
    if (!path) {
      return new Response("Path is required", { status: 400 });
    }
    try {
      const info = await stat(path);
      return { ok: info.isFile() };
    } catch {
      return { ok: false };
    }
  })
  .ws("/api/stream", {
    open(ws) {
      wsClients.add(ws);
      ws.send(JSON.stringify({ type: "status", data: getProcessStatus() }));
    },
    close(ws) {
      wsClients.delete(ws);
    },
    message(ws, message) {
      if (message === "ping") {
        ws.send("pong");
      }
    },
  })
  .get("/static/*", ({ params }) => {
    const file = Bun.file(resolve(staticDir, params["*"]));
    return new Response(file);
  })
  .get("/*", () => new Response(Bun.file(indexFile)))
  .listen({ hostname: appHost, port: appPort });

console.log(`alicebot-dashboard listening on http://${appHost}:${appPort}`);
