export function initPerformanceTelemetry() {
  if (typeof window === "undefined" || !("PerformanceObserver" in window)) {
    return;
  }

  const baseUrl = import.meta.env.VITE_CONTROL_API_BASE_URL ?? "";

  const sendMetric = (metric: string, value: number) => {
    fetch(`${baseUrl}/v1/telemetry/performance`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        metric,
        value,
        path: window.location.pathname,
      }),
      keepalive: true,
    }).catch(() => { /* silent fail */ });
  };

  try {
    // FCP
    const fcpObserver = new PerformanceObserver((entryList) => {
      const entries = entryList.getEntries();
      if (entries.length > 0) {
        sendMetric("FCP", entries[entries.length - 1].startTime);
        fcpObserver.disconnect();
      }
    });
    fcpObserver.observe({ type: "paint", buffered: true });

    // LCP
    const lcpObserver = new PerformanceObserver((entryList) => {
      const entries = entryList.getEntries();
      if (entries.length > 0) {
        sendMetric("LCP", entries[entries.length - 1].startTime);
      }
    });
    lcpObserver.observe({ type: "largest-contentful-paint", buffered: true });

    // CLS
    let clsValue = 0;
    const clsObserver = new PerformanceObserver((entryList) => {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      for (const entry of entryList.getEntries() as any[]) {
        if (!entry.hadRecentInput) {
          clsValue += entry.value;
          sendMetric("CLS", clsValue);
        }
      }
    });
    clsObserver.observe({ type: "layout-shift", buffered: true });
  } catch {
    // Ignore observer setup errors
  }
}
