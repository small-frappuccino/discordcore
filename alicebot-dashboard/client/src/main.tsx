import React, { useEffect, useMemo, useState } from "react";
import { createRoot } from "react-dom/client";
import "./styles.css";

type LogLevel = "INFO" | "WARN" | "ERROR" | "DEBUG";

type EventLog = {
  ts: string;
  level: LogLevel;
  category: string;
  message: string;
  guildId?: string;
  userId?: string;
  channelId?: string;
  meta?: Record<string, string>;
  stream: "stdout" | "stderr";
};

type ProcessStatus = {
  running: boolean;
  pid?: number;
  startedAt?: string;
  lastExitCode?: number;
  lastExitSignal?: string;
  executablePath: string;
};

type FeatureSettings = {
  services: {
    monitoring: boolean;
    automod: boolean;
    commands: boolean;
    adminCommands: boolean;
  };
  logging: {
    message: boolean;
    entryExit: boolean;
    reaction: boolean;
    user: boolean;
    automod: boolean;
    clean: boolean;
    moderation: boolean;
  };
  messageCache: {
    cleanupOnStartup: boolean;
    deleteOnLog: boolean;
  };
  presenceWatch: {
    bot: boolean;
  };
  maintenance: {
    dbCleanup: boolean;
  };
  safety: {
    botRolePermMirror: boolean;
  };
  backfill: {
    enabled: boolean;
  };
};

type GuildFeatureSettings = {
  monitoring: boolean;
  automod: boolean;
  statsChannels: boolean;
  autoRoleAssignment: boolean;
  userPrune: boolean;
};

type GuildSettings = {
  id: string;
  name?: string;
  enabled: boolean;
  features: GuildFeatureSettings;
  notificationChannelId?: string;
};

type Settings = {
  executablePath: string;
  guilds: GuildSettings[];
  features: FeatureSettings;
};

type ServiceStatus = { id: string; name: string; status: string };

type ApiStatus = {
  bot: { connected: boolean; username: string; guildCount: number; uptimeSeconds: number };
  process: ProcessStatus;
};

const pages = [
  "Overview",
  "Guilds",
  "Monitoring",
  "Automod",
  "Commands",
  "Logs",
  "Settings",
] as const;

type Page = (typeof pages)[number];

const useAdminToken = () => {
  const [token, setToken] = useState(() => localStorage.getItem("adminToken") ?? "");
  useEffect(() => {
    localStorage.setItem("adminToken", token);
  }, [token]);
  return [token, setToken] as const;
};

const fetchJson = async <T,>(input: RequestInfo, init?: RequestInit): Promise<T> => {
  const response = await fetch(input, init);
  if (!response.ok) {
    throw new Error(await response.text());
  }
  return (await response.json()) as T;
};

const Card: React.FC<{ title: string; children: React.ReactNode }> = ({ title, children }) => (
  <div className="card">
    <div className="card-title">{title}</div>
    <div className="card-body">{children}</div>
  </div>
);

const Badge: React.FC<{ variant: "success" | "warning" | "danger" | "neutral"; label: string }> = ({
  variant,
  label,
}) => <span className={`badge badge-${variant}`}>{label}</span>;

const Toggle: React.FC<{ checked: boolean; onChange: (value: boolean) => void }> = ({
  checked,
  onChange,
}) => (
  <button className={`toggle ${checked ? "on" : "off"}`} onClick={() => onChange(!checked)}>
    <span className="toggle-handle" />
  </button>
);

const Table: React.FC<{ headers: string[]; children: React.ReactNode }> = ({ headers, children }) => (
  <table className="table">
    <thead>
      <tr>
        {headers.map((header) => (
          <th key={header}>{header}</th>
        ))}
      </tr>
    </thead>
    <tbody>{children}</tbody>
  </table>
);

const Toast: React.FC<{ message: string; variant?: "success" | "error" }> = ({
  message,
  variant = "success",
}) => <div className={`toast ${variant}`}>{message}</div>;

const App = () => {
  const [page, setPage] = useState<Page>("Overview");
  const [status, setStatus] = useState<ApiStatus | null>(null);
  const [services, setServices] = useState<ServiceStatus[]>([]);
  const [settings, setSettings] = useState<Settings | null>(null);
  const [logs, setLogs] = useState<EventLog[]>([]);
  const [toast, setToast] = useState<{ message: string; variant?: "success" | "error" } | null>(null);
  const [adminToken, setAdminToken] = useAdminToken();

  const processStatus = status?.process;

  const showToast = (message: string, variant: "success" | "error" = "success") => {
    setToast({ message, variant });
    setTimeout(() => setToast(null), 3000);
  };

  const loadStatus = async () => {
    try {
      const data = await fetchJson<ApiStatus>("/api/status");
      setStatus(data);
    } catch (error) {
      console.error(error);
    }
  };

  const loadSettings = async () => {
    try {
      const data = await fetchJson<Settings>("/api/settings");
      setSettings(data);
    } catch (error) {
      console.error(error);
    }
  };

  const loadServices = async () => {
    try {
      const data = await fetchJson<ServiceStatus[]>("/api/services");
      setServices(data);
    } catch (error) {
      console.error(error);
    }
  };

  const loadLogs = async () => {
    try {
      const data = await fetchJson<EventLog[]>("/api/logs?limit=200");
      setLogs(data);
    } catch (error) {
      console.error(error);
    }
  };

  useEffect(() => {
    void loadStatus();
    void loadSettings();
    void loadServices();
    void loadLogs();
    const interval = setInterval(loadStatus, 5000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    let ws: WebSocket | null = null;
    let timer: number | null = null;

    const connect = () => {
      ws = new WebSocket(`ws://${window.location.host}/api/stream`);
      ws.onmessage = (event) => {
        try {
          const payload = JSON.parse(event.data);
          if (payload.type === "log") {
            setLogs((prev) => [...prev.slice(-199), payload.data]);
          }
          if (payload.type === "status") {
            setStatus((prev) => (prev ? { ...prev, process: payload.data } : prev));
          }
        } catch {
          // ignore
        }
      };
      ws.onclose = () => {
        timer = window.setTimeout(connect, 1500);
      };
    };

    connect();

    return () => {
      ws?.close();
      if (timer) window.clearTimeout(timer);
    };
  }, []);

  const handleProcess = async (action: "start" | "stop" | "restart") => {
    try {
      const response = await fetchJson<ProcessStatus>(`/api/process/${action}`, {
        method: "POST",
        headers: { "X-Admin-Token": adminToken },
      });
      setStatus((prev) => (prev ? { ...prev, process: response } : prev));
      showToast(`Process ${action} issued`);
    } catch (error) {
      showToast(String(error), "error");
    }
  };

  const updateSettings = async (next: Settings) => {
    try {
      const response = await fetchJson<Settings>("/api/settings", {
        method: "PUT",
        headers: {
          "Content-Type": "application/json",
          "X-Admin-Token": adminToken,
        },
        body: JSON.stringify(next),
      });
      setSettings(response);
      showToast("Settings saved");
    } catch (error) {
      showToast(String(error), "error");
    }
  };

  const uptimeLabel = useMemo(() => {
    if (!processStatus?.running || !processStatus.startedAt) return "--";
    const started = new Date(processStatus.startedAt).getTime();
    const diff = Date.now() - started;
    const hours = Math.floor(diff / 3600000);
    const minutes = Math.floor((diff % 3600000) / 60000);
    const seconds = Math.floor((diff % 60000) / 1000);
    return `${hours}h ${minutes}m ${seconds}s`;
  }, [processStatus]);

  return (
    <div className="app">
      <aside className="sidebar">
        <div className="brand">
          <div className="brand-logo">A</div>
          <div>
            <div className="brand-title">alicebot</div>
            <div className="brand-subtitle">dashboard</div>
          </div>
        </div>
        <nav>
          {pages.map((item) => (
            <button
              key={item}
              className={`nav-item ${page === item ? "active" : ""}`}
              onClick={() => setPage(item)}
            >
              {item}
            </button>
          ))}
        </nav>
      </aside>
      <main className="main">
        <header className="topbar">
          <div className="topbar-left">
            <Badge
              variant={processStatus?.running ? "success" : "danger"}
              label={processStatus?.running ? "Running" : "Stopped"}
            />
            <div className="uptime">Uptime: {uptimeLabel}</div>
          </div>
          <div className="topbar-actions">
            <button className="btn" onClick={() => handleProcess("start")}>Start</button>
            <button className="btn" onClick={() => handleProcess("stop")}>Stop</button>
            <button className="btn btn-primary" onClick={() => handleProcess("restart")}>
              Restart
            </button>
            <div className="discord-status">
              <span>Connected to Discord</span>
              <Badge variant={status?.bot.connected ? "success" : "warning"} label={status?.bot.connected ? "Online" : "Mock"} />
            </div>
          </div>
        </header>

        <section className="content">
          {page === "Overview" && (
            <OverviewPage
              status={status}
              services={services}
              logs={logs}
              onRefresh={loadStatus}
            />
          )}
          {page === "Guilds" && settings && (
            <GuildsPage settings={settings} onUpdate={updateSettings} />
          )}
          {page === "Monitoring" && settings && (
            <MonitoringPage settings={settings} onUpdate={updateSettings} />
          )}
          {page === "Automod" && settings && (
            <AutomodPage settings={settings} onUpdate={updateSettings} />
          )}
          {page === "Commands" && <CommandsPage />}
          {page === "Logs" && <LogsPage logs={logs} />}
          {page === "Settings" && settings && (
            <SettingsPage
              settings={settings}
              onUpdate={updateSettings}
              adminToken={adminToken}
              setAdminToken={setAdminToken}
            />
          )}
        </section>
      </main>
      {toast && <Toast message={toast.message} variant={toast.variant} />}
    </div>
  );
};

const OverviewPage: React.FC<{
  status: ApiStatus | null;
  services: ServiceStatus[];
  logs: EventLog[];
  onRefresh: () => void;
}> = ({ status, services, logs, onRefresh }) => (
  <div className="grid">
    <Card title="Process Status">
      <div className="stat">
        <strong>Status:</strong> {status?.process.running ? "Running" : "Stopped"}
      </div>
      <div className="stat">
        <strong>PID:</strong> {status?.process.pid ?? "--"}
      </div>
      <button className="btn" onClick={onRefresh}>
        Refresh
      </button>
    </Card>
    <Card title="Guilds">
      <div className="stat">
        <strong>Total:</strong> {status?.bot.guildCount ?? 0}
      </div>
    </Card>
    <Card title="Services">
      {services.map((service) => (
        <div className="service" key={service.id}>
          <span>{service.name}</span>
          <Badge
            variant={service.status === "healthy" ? "success" : "warning"}
            label={service.status}
          />
        </div>
      ))}
    </Card>
    <Card title="Recent Activity">
      <div className="log-preview">
        {logs.slice(-5).map((log, index) => (
          <div key={`${log.ts}-${index}`} className="log-line">
            [{log.level}] {log.message}
          </div>
        ))}
      </div>
    </Card>
  </div>
);

const GuildsPage: React.FC<{ settings: Settings; onUpdate: (s: Settings) => void }> = ({
  settings,
  onUpdate,
}) => (
  <div>
    <h2>Guilds</h2>
    <Table
      headers={[
        "Guild",
        "Enabled",
        "Monitoring",
        "Automod",
        "Stats",
        "Auto Role",
        "User Prune",
      ]}
    >
      {settings.guilds.map((guild) => (
        <tr key={guild.id}>
          <td>
            <div className="guild-name">{guild.name ?? guild.id}</div>
            <div className="muted">{guild.id}</div>
          </td>
          <td>
            <Toggle
              checked={guild.enabled}
              onChange={(value) =>
                onUpdate({
                  ...settings,
                  guilds: settings.guilds.map((item) =>
                    item.id === guild.id ? { ...item, enabled: value } : item
                  ),
                })
              }
            />
          </td>
          <td>
            <Toggle
              checked={guild.features.monitoring}
              onChange={(value) =>
                onUpdate({
                  ...settings,
                  guilds: settings.guilds.map((item) =>
                    item.id === guild.id
                      ? {
                          ...item,
                          features: { ...item.features, monitoring: value },
                        }
                      : item
                  ),
                })
              }
            />
          </td>
          <td>
            <Toggle
              checked={guild.features.automod}
              onChange={(value) =>
                onUpdate({
                  ...settings,
                  guilds: settings.guilds.map((item) =>
                    item.id === guild.id
                      ? {
                          ...item,
                          features: { ...item.features, automod: value },
                        }
                      : item
                  ),
                })
              }
            />
          </td>
          <td>
            <Toggle
              checked={guild.features.statsChannels}
              onChange={(value) =>
                onUpdate({
                  ...settings,
                  guilds: settings.guilds.map((item) =>
                    item.id === guild.id
                      ? {
                          ...item,
                          features: { ...item.features, statsChannels: value },
                        }
                      : item
                  ),
                })
              }
            />
          </td>
          <td>
            <Toggle
              checked={guild.features.autoRoleAssignment}
              onChange={(value) =>
                onUpdate({
                  ...settings,
                  guilds: settings.guilds.map((item) =>
                    item.id === guild.id
                      ? {
                          ...item,
                          features: { ...item.features, autoRoleAssignment: value },
                        }
                      : item
                  ),
                })
              }
            />
          </td>
          <td>
            <Toggle
              checked={guild.features.userPrune}
              onChange={(value) =>
                onUpdate({
                  ...settings,
                  guilds: settings.guilds.map((item) =>
                    item.id === guild.id
                      ? {
                          ...item,
                          features: { ...item.features, userPrune: value },
                        }
                      : item
                  ),
                })
              }
            />
          </td>
        </tr>
      ))}
    </Table>
  </div>
);

const MonitoringPage: React.FC<{ settings: Settings; onUpdate: (s: Settings) => void }> = ({
  settings,
  onUpdate,
}) => (
  <div>
    <h2>Monitoring</h2>
    <div className="form-row">
      <label>Monitoring Enabled</label>
      <Toggle
        checked={settings.features.services.monitoring}
        onChange={(value) =>
          onUpdate({
            ...settings,
            features: {
              ...settings.features,
              services: { ...settings.features.services, monitoring: value },
            },
          })
        }
      />
    </div>
    <Table headers={["Guild", "Monitoring", "Notification Channel"]}>
      {settings.guilds.map((guild) => (
        <tr key={guild.id}>
          <td>{guild.name ?? guild.id}</td>
          <td>
            <Toggle
              checked={guild.features.monitoring}
              onChange={(value) =>
                onUpdate({
                  ...settings,
                  guilds: settings.guilds.map((item) =>
                    item.id === guild.id
                      ? {
                          ...item,
                          features: { ...item.features, monitoring: value },
                        }
                      : item
                  ),
                })
              }
            />
          </td>
          <td>
            <input
              className="input"
              value={guild.notificationChannelId ?? ""}
              placeholder="Channel ID"
              onChange={(event) =>
                onUpdate({
                  ...settings,
                  guilds: settings.guilds.map((item) =>
                    item.id === guild.id
                      ? { ...item, notificationChannelId: event.target.value }
                      : item
                  ),
                })
              }
            />
          </td>
        </tr>
      ))}
    </Table>
  </div>
);

const AutomodPage: React.FC<{ settings: Settings; onUpdate: (s: Settings) => void }> = ({
  settings,
  onUpdate,
}) => (
  <div>
    <h2>Automod</h2>
    <div className="form-row">
      <label>Automod Enabled</label>
      <Toggle
        checked={settings.features.services.automod}
        onChange={(value) =>
          onUpdate({
            ...settings,
            features: {
              ...settings.features,
              services: { ...settings.features.services, automod: value },
            },
          })
        }
      />
    </div>
    <Card title="Rules (placeholder)">
      <ul>
        <li>Anti-spam cooldown</li>
        <li>Caps lock limiter</li>
        <li>Invite link blocker</li>
      </ul>
    </Card>
  </div>
);

const CommandsPage: React.FC = () => (
  <div>
    <h2>Commands</h2>
    <Card title="Available Commands">
      <ul className="command-list">
        <li>/ping</li>
        <li>/echo</li>
        <li>/metrics</li>
        <li>/moderation</li>
        <li>/config</li>
      </ul>
    </Card>
  </div>
);

const LogsPage: React.FC<{ logs: EventLog[] }> = ({ logs }) => {
  const [levelFilter, setLevelFilter] = useState<LogLevel | "ALL">("ALL");
  const [categoryFilter, setCategoryFilter] = useState<string>("");
  const [search, setSearch] = useState<string>("");
  const [autoScroll, setAutoScroll] = useState(true);

  const filtered = useMemo(() => {
    return logs.filter((log) => {
      if (levelFilter !== "ALL" && log.level !== levelFilter) return false;
      if (categoryFilter && log.category !== categoryFilter) return false;
      if (search && !log.message.toLowerCase().includes(search.toLowerCase())) return false;
      return true;
    });
  }, [logs, levelFilter, categoryFilter, search]);

  useEffect(() => {
    if (autoScroll) {
      const container = document.querySelector(".log-console");
      if (container) {
        container.scrollTop = container.scrollHeight;
      }
    }
  }, [filtered, autoScroll]);

  const categories = Array.from(new Set(logs.map((log) => log.category)));

  return (
    <div>
      <h2>Logs</h2>
      <div className="filters">
        <select value={levelFilter} onChange={(event) => setLevelFilter(event.target.value as LogLevel | "ALL")}>
          <option value="ALL">All levels</option>
          <option value="INFO">INFO</option>
          <option value="WARN">WARN</option>
          <option value="ERROR">ERROR</option>
          <option value="DEBUG">DEBUG</option>
        </select>
        <select value={categoryFilter} onChange={(event) => setCategoryFilter(event.target.value)}>
          <option value="">All categories</option>
          {categories.map((category) => (
            <option key={category} value={category}>
              {category}
            </option>
          ))}
        </select>
        <input
          className="input"
          placeholder="Search logs"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
        />
        <label className="inline-toggle">
          <input type="checkbox" checked={autoScroll} onChange={(event) => setAutoScroll(event.target.checked)} />
          Auto-scroll
        </label>
      </div>
      <div className="log-console">
        {filtered.length === 0 && <div className="empty">No logs yet.</div>}
        {filtered.map((log, index) => (
          <div key={`${log.ts}-${index}`} className={`log-line log-${log.level.toLowerCase()}`}>
            <span className="log-ts">{new Date(log.ts).toLocaleTimeString()}</span>
            <span className="log-level">[{log.level}]</span>
            <span className="log-category">[{log.category}]</span>
            <span className="log-message">{log.message}</span>
          </div>
        ))}
      </div>
    </div>
  );
};

const SettingsPage: React.FC<{
  settings: Settings;
  onUpdate: (s: Settings) => void;
  adminToken: string;
  setAdminToken: (value: string) => void;
}> = ({ settings, onUpdate, adminToken, setAdminToken }) => {
  const [path, setPath] = useState(settings.executablePath);
  const [validation, setValidation] = useState<string>("");

  const testPath = async () => {
    try {
      const response = await fetchJson<{ ok: boolean }>("/api/process/validate-path", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "X-Admin-Token": adminToken,
        },
        body: JSON.stringify({ path }),
      });
      setValidation(response.ok ? "Path ok" : "Path not found");
    } catch (error) {
      setValidation(String(error));
    }
  };

  return (
    <div>
      <h2>Settings</h2>
      <div className="form-grid">
        <label>Admin Token</label>
        <input
          className="input"
          value={adminToken}
          onChange={(event) => setAdminToken(event.target.value)}
          placeholder="X-Admin-Token"
        />
        <label>Bot executable path</label>
        <div className="inline">
          <input
            className="input"
            value={path}
            onChange={(event) => setPath(event.target.value)}
            placeholder="C:\\Users\\alice\\.local\\bin\\alicebot.exe"
          />
          <button className="btn" onClick={testPath}>
            Test path
          </button>
        </div>
        <div className="hint">{validation}</div>
      </div>
      <button
        className="btn btn-primary"
        onClick={() => onUpdate({ ...settings, executablePath: path })}
      >
        Save Settings
      </button>
      <Card title="Feature Toggles">
        <div className="form-grid">
          <label>Monitoring service</label>
          <Toggle
            checked={settings.features.services.monitoring}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  services: { ...settings.features.services, monitoring: value },
                },
              })
            }
          />
          <label>Automod service</label>
          <Toggle
            checked={settings.features.services.automod}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  services: { ...settings.features.services, automod: value },
                },
              })
            }
          />
          <label>Commands</label>
          <Toggle
            checked={settings.features.services.commands}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  services: { ...settings.features.services, commands: value },
                },
              })
            }
          />
          <label>Admin commands</label>
          <Toggle
            checked={settings.features.services.adminCommands}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  services: { ...settings.features.services, adminCommands: value },
                },
              })
            }
          />
          <label>Message logs</label>
          <Toggle
            checked={settings.features.logging.message}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  logging: { ...settings.features.logging, message: value },
                },
              })
            }
          />
          <label>Entry/exit logs</label>
          <Toggle
            checked={settings.features.logging.entryExit}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  logging: { ...settings.features.logging, entryExit: value },
                },
              })
            }
          />
          <label>Reaction logs</label>
          <Toggle
            checked={settings.features.logging.reaction}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  logging: { ...settings.features.logging, reaction: value },
                },
              })
            }
          />
          <label>User logs</label>
          <Toggle
            checked={settings.features.logging.user}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  logging: { ...settings.features.logging, user: value },
                },
              })
            }
          />
          <label>Automod logs</label>
          <Toggle
            checked={settings.features.logging.automod}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  logging: { ...settings.features.logging, automod: value },
                },
              })
            }
          />
          <label>Clean log</label>
          <Toggle
            checked={settings.features.logging.clean}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  logging: { ...settings.features.logging, clean: value },
                },
              })
            }
          />
          <label>Moderation logs</label>
          <Toggle
            checked={settings.features.logging.moderation}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  logging: { ...settings.features.logging, moderation: value },
                },
              })
            }
          />
          <label>Message cache cleanup</label>
          <Toggle
            checked={settings.features.messageCache.cleanupOnStartup}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  messageCache: {
                    ...settings.features.messageCache,
                    cleanupOnStartup: value,
                  },
                },
              })
            }
          />
          <label>Message delete on log</label>
          <Toggle
            checked={settings.features.messageCache.deleteOnLog}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  messageCache: {
                    ...settings.features.messageCache,
                    deleteOnLog: value,
                  },
                },
              })
            }
          />
          <label>Presence watch (bot)</label>
          <Toggle
            checked={settings.features.presenceWatch.bot}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  presenceWatch: { ...settings.features.presenceWatch, bot: value },
                },
              })
            }
          />
          <label>DB cleanup</label>
          <Toggle
            checked={settings.features.maintenance.dbCleanup}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  maintenance: { ...settings.features.maintenance, dbCleanup: value },
                },
              })
            }
          />
          <label>Bot role perm mirror</label>
          <Toggle
            checked={settings.features.safety.botRolePermMirror}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  safety: { ...settings.features.safety, botRolePermMirror: value },
                },
              })
            }
          />
          <label>Entry/exit backfill</label>
          <Toggle
            checked={settings.features.backfill.enabled}
            onChange={(value) =>
              onUpdate({
                ...settings,
                features: {
                  ...settings.features,
                  backfill: { ...settings.features.backfill, enabled: value },
                },
              })
            }
          />
        </div>
      </Card>
      <Card title="Settings JSON">
        <pre className="json-view">{JSON.stringify(settings, null, 2)}</pre>
      </Card>
    </div>
  );
};

const container = document.getElementById("root");
if (container) {
  createRoot(container).render(<App />);
}



