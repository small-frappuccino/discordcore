interface LogPayload {
  level: "info" | "warn" | "error";
  message: string;
  path: string;
  context?: Record<string, unknown>;
}

class TelemetryLogger {
  private static instance: TelemetryLogger;
  private buffer: LogPayload[] = [];
  private flushTimeout: number | null = null;
  private isUnloading = false;

  private constructor() {
    if (typeof window !== "undefined") {
      window.addEventListener("beforeunload", () => {
        this.isUnloading = true;
        this.flush(true);
      });
    }
  }

  static getInstance(): TelemetryLogger {
    if (!TelemetryLogger.instance) {
      TelemetryLogger.instance = new TelemetryLogger();
    }
    return TelemetryLogger.instance;
  }

  private enqueue(payload: LogPayload) {
    this.buffer.push(payload);
    if (!this.flushTimeout && !this.isUnloading) {
      this.flushTimeout = window.setTimeout(() => this.flush(), 2000);
    }
  }

  info(message: string, context?: Record<string, unknown>) {
    console.info(message, context);
    this.enqueue({ level: "info", message, path: window.location.pathname, context });
  }

  warn(message: string, context?: Record<string, unknown>) {
    console.warn(message, context);
    this.enqueue({ level: "warn", message, path: window.location.pathname, context });
  }

  error(message: string, context?: Record<string, unknown>) {
    console.error(message, context);
    this.enqueue({ level: "error", message, path: window.location.pathname, context });
  }

  private flush(sync = false) {
    if (this.flushTimeout) {
      clearTimeout(this.flushTimeout);
      this.flushTimeout = null;
    }
    
    if (this.buffer.length === 0) return;

    const payload = [...this.buffer];
    this.buffer = [];

    // Assuming same origin due to Vite proxy
    const baseUrl = import.meta.env.VITE_CONTROL_API_BASE_URL ?? "";

    payload.forEach(log => {
      if (sync && navigator.sendBeacon) {
        navigator.sendBeacon(`${baseUrl}/v1/telemetry/logs`, JSON.stringify(log));
      } else {
        fetch(`${baseUrl}/v1/telemetry/logs`, {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(log),
          keepalive: true,
        }).catch(() => { /* silent fail for telemetry */ });
      }
    });
  }
}

export const logger = TelemetryLogger.getInstance();
