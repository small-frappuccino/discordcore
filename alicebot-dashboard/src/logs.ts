import type { EventLog } from "./types";

export class LogStore {
  private logs: EventLog[] = [];
  private listeners = new Set<(log: EventLog) => void>();

  constructor(private maxSize = 5000) {}

  add(log: EventLog) {
    this.logs.push(log);
    if (this.logs.length > this.maxSize) {
      this.logs.splice(0, this.logs.length - this.maxSize);
    }
    for (const listener of this.listeners) {
      listener(log);
    }
  }

  list(limit = 200): EventLog[] {
    return this.logs.slice(-limit);
  }

  onLog(listener: (log: EventLog) => void) {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }
}
