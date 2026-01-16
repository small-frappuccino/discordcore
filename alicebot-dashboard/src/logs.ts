import type { EventLog } from "./types";

export class LogStore {
  private logs: EventLog[] = [];
  private listeners = new Set<(log: EventLog) => void>();
  private head = 0;
  private size = 0;

  constructor(private maxSize = 5000) {}

  add(log: EventLog) {
    if (this.maxSize <= 0) {
      this.logs.push(log);
    } else if (this.size < this.maxSize) {
      this.logs.push(log);
      this.size++;
      this.head = this.size % this.maxSize;
    } else {
      this.logs[this.head] = log;
      this.head = (this.head + 1) % this.maxSize;
    }

    for (const listener of this.listeners) {
      listener(log);
    }
  }

  list(limit = 200): EventLog[] {
    if (this.maxSize <= 0) {
      return this.logs.slice(-limit);
    }
    if (this.size === 0) {
      return [];
    }
    if (this.size < this.maxSize) {
      return this.logs.slice(-limit);
    }

    const ordered = new Array<EventLog>(this.size);
    const firstSpan = this.logs.length - this.head;
    ordered.splice(0, firstSpan, ...this.logs.slice(this.head));
    ordered.splice(firstSpan, this.head, ...this.logs.slice(0, this.head));
    return ordered.slice(-limit);
  }

  onLog(listener: (log: EventLog) => void) {
    this.listeners.add(listener);
    return () => this.listeners.delete(listener);
  }
}
