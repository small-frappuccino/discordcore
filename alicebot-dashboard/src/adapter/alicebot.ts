import type { EventLog, ProcessStatus } from "../types";

type LogSink = (log: EventLog) => void;

type StatusSink = (status: ProcessStatus) => void;

const parseLogLine = (line: string, stream: "stdout" | "stderr"): EventLog => {
  const now = new Date().toISOString();
  if (line.includes("time=") && line.includes("level=") && line.includes("msg=")) {
    const tokens = Array.from(line.matchAll(/(\w+)=(("[^"]*")|(\S+))/g));
    const map: Record<string, string> = {};
    for (const token of tokens) {
      const key = token[1];
      const rawValue = token[2];
      map[key] = rawValue.startsWith('"')
        ? rawValue.slice(1, -1)
        : rawValue;
    }

    const meta: Record<string, string> = {};
    for (const [key, value] of Object.entries(map)) {
      if (["time", "level", "category", "msg"].includes(key)) {
        continue;
      }
      meta[key] = value;
    }

    return {
      ts: map.time ?? now,
      level: (map.level?.toUpperCase() ?? "INFO") as EventLog["level"],
      category: map.category ?? "process",
      message: map.msg ?? line,
      guildId: map.guildID ?? map.guildId,
      userId: map.userID ?? map.userId,
      channelId: map.channelID ?? map.channelId,
      meta: Object.keys(meta).length ? meta : undefined,
      stream,
    };
  }

  return {
    ts: now,
    level: "INFO",
    category: "process",
    message: line,
    stream,
  };
};

const streamLines = async (
  stream: ReadableStream<Uint8Array> | null,
  streamType: "stdout" | "stderr",
  sink: LogSink
) => {
  if (!stream) return;
  const reader = stream.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  while (true) {
    const { done, value } = await reader.read();
    if (done) {
      if (buffer.trim().length) {
        sink(parseLogLine(buffer.trim(), streamType));
      }
      break;
    }
    buffer += decoder.decode(value, { stream: true });
    let index = buffer.indexOf("\n");
    while (index !== -1) {
      const line = buffer.slice(0, index).trim();
      if (line.length) {
        sink(parseLogLine(line, streamType));
      }
      buffer = buffer.slice(index + 1);
      index = buffer.indexOf("\n");
    }
  }
};

export class AliceBotAdapter {
  private process: Bun.Subprocess | null = null;
  private status: ProcessStatus;

  constructor(
    private getExecutablePath: () => string,
    private logSink: LogSink,
    private statusSink: StatusSink
  ) {
    this.status = {
      running: false,
      executablePath: this.getExecutablePath(),
    };
  }

  getStatus(): ProcessStatus {
    return { ...this.status, executablePath: this.getExecutablePath() };
  }

  async start(): Promise<ProcessStatus> {
    if (this.process) {
      return this.getStatus();
    }

    const executablePath = this.getExecutablePath();
    const proc = Bun.spawn({
      cmd: [executablePath],
      stdout: "pipe",
      stderr: "pipe",
    });

    this.process = proc;
    this.status = {
      running: true,
      pid: proc.pid,
      startedAt: new Date().toISOString(),
      executablePath,
    };
    this.statusSink(this.getStatus());

    void streamLines(proc.stdout, "stdout", this.logSink);
    void streamLines(proc.stderr, "stderr", this.logSink);

    proc.exited.then(({ code, signal }) => {
      this.process = null;
      this.status = {
        running: false,
        startedAt: this.status.startedAt,
        executablePath,
        lastExitCode: code ?? undefined,
        lastExitSignal: signal ?? undefined,
      };
      this.statusSink(this.getStatus());
    });

    return this.getStatus();
  }

  async stop(): Promise<ProcessStatus> {
    if (!this.process) {
      return this.getStatus();
    }

    const current = this.process;
    try {
      current.kill();
    } catch {
      // ignore
    }

    await new Promise((resolve) => setTimeout(resolve, 300));

    if (this.process) {
      const pid = this.process.pid;
      try {
        if (process.platform === "win32" && pid) {
          Bun.spawn({
            cmd: ["cmd", "/c", "taskkill", "/PID", String(pid), "/T", "/F"],
            stdout: "ignore",
            stderr: "ignore",
          });
        }
      } catch {
        // ignore
      }
    }

    await current.exited.catch(() => undefined);
    return this.getStatus();
  }

  async restart(): Promise<ProcessStatus> {
    await this.stop();
    return this.start();
  }
}
