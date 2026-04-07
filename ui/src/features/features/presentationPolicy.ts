import type { ReactNode } from "react";

export interface DashboardMetaItem {
  label: string;
  value: ReactNode;
}

const dashboardDiagnosticQueryParam = "diagnostics";

const blacklistedMetaLabels = new Set([
  "account",
  "active account",
  "base url",
  "environment",
  "hostname",
  "origin",
  "selected server",
  "server",
]);

const blacklistedKeyValueLabels = new Set([
  "applied from",
  "effective source",
  "inherited source",
  "override",
  "server setting",
]);

const blacklistedValuePatterns = [
  /^configured here$/i,
  /^using configured setting$/i,
  /^using default(?: setting)?$/i,
  /^using inherited setting$/i,
  /^enabled for this server$/i,
  /^disabled for this server$/i,
];

const diagnosticFieldPatterns = [/\bid fallback\b/i, /\braw id\b/i];

const blacklistedFieldNotePatterns = [
  /^use only if .* lookup fails\.?$/i,
  /^use this only when .* paste .* id directly\.?$/i,
];

export function isDashboardDiagnosticModeEnabled() {
  if (typeof window === "undefined") {
    return false;
  }

  try {
    return (
      new URLSearchParams(window.location.search).get(
        dashboardDiagnosticQueryParam,
      ) === "1"
    );
  } catch {
    return false;
  }
}

export function getVisibleDashboardMetaItems(
  items: DashboardMetaItem[],
  diagnosticMode = isDashboardDiagnosticModeEnabled(),
) {
  if (diagnosticMode) {
    return items;
  }

  return items.filter(
    (item) => !blacklistedMetaLabels.has(normalizePolicyText(item.label)),
  );
}

export function filterDashboardKeyValueItems<
  T extends { label: ReactNode; value: ReactNode },
>(
  items: T[],
  diagnosticMode = isDashboardDiagnosticModeEnabled(),
) {
  if (diagnosticMode) {
    return items;
  }

  return items.filter((item) => {
    const labelText = readPolicyText(item.label);
    if (
      labelText !== "" &&
      blacklistedKeyValueLabels.has(normalizePolicyText(labelText))
    ) {
      return false;
    }

    const valueText = readPolicyText(item.value);
    if (
      valueText !== "" &&
      blacklistedValuePatterns.some((pattern) => pattern.test(valueText))
    ) {
      return false;
    }

    return true;
  });
}

export function shouldRenderDashboardDiagnosticField(
  label: string,
  diagnosticMode = isDashboardDiagnosticModeEnabled(),
) {
  if (diagnosticMode) {
    return true;
  }

  return !diagnosticFieldPatterns.some((pattern) => pattern.test(label));
}

export function sanitizeDashboardFieldNote(
  label: string,
  note: ReactNode,
  diagnosticMode = isDashboardDiagnosticModeEnabled(),
) {
  if (diagnosticMode || typeof note !== "string") {
    return note;
  }

  if (!shouldRenderDashboardDiagnosticField(label, diagnosticMode)) {
    return null;
  }

  return blacklistedFieldNotePatterns.some((pattern) => pattern.test(note.trim()))
    ? null
    : note;
}

function readPolicyText(value: ReactNode) {
  if (typeof value === "string" || typeof value === "number") {
    return String(value).trim();
  }

  return "";
}

function normalizePolicyText(value: string) {
  return value.trim().toLowerCase().replace(/\s+/g, " ");
}
