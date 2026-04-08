import {
  useId,
  useState,
  type CSSProperties,
  type HTMLAttributes,
  type ReactNode,
} from "react";
import { getInitials } from "../app/utils";
import type { Notice } from "../app/types";
import {
  filterDashboardKeyValueItems,
  getVisibleDashboardMetaItems,
  sanitizeDashboardFieldNote,
  shouldRenderDashboardDiagnosticField,
  type DashboardMetaItem,
} from "../features/features/presentationPolicy";
export {
  GroupedSettingsBlock,
  GroupedSettingsCopy,
  GroupedSettingsGroup,
  GroupedSettingsHeading,
  GroupedSettingsInlineMessage,
  GroupedSettingsItem,
  GroupedSettingsMainRow,
  GroupedSettingsRow,
  GroupedSettingsSection,
  GroupedSettingsStack,
  GroupedSettingsSubrow,
  GroupedSettingsSwitch,
} from "./groupedSettings";

type StatusTone = "neutral" | "info" | "success" | "error";
type SurfaceElement = "article" | "div" | "section";

interface PageHeaderProps {
  eyebrow?: string;
  title: string;
  description: string;
  status?: ReactNode;
  meta?: ReactNode;
  actions?: ReactNode;
}

interface DashboardMetaListProps {
  items: DashboardMetaItem[];
}

interface EmptyStateProps {
  title: string;
  description: string;
  action?: ReactNode;
}

interface IdentityAvatarProps {
  imageUrl?: string | null;
  label: string;
}

interface AlertBannerProps {
  notice?: Notice | null;
  busyLabel?: string;
}

interface UnsavedChangesBarProps {
  hasUnsavedChanges: boolean;
  message?: ReactNode;
  saveLabel?: string;
  resetLabel?: string;
  onSave: () => void | Promise<void>;
  onReset: () => void | Promise<void>;
  saving?: boolean;
  disabled?: boolean;
  className?: string;
}

interface LookupNoticeProps {
  title: string;
  message: ReactNode;
  retryLabel?: string;
  onRetry?: (() => void | Promise<void>) | null;
  retryDisabled?: boolean;
  as?: SurfaceElement;
}

interface StatusBadgeProps {
  tone?: StatusTone;
  children: ReactNode;
}

interface SurfaceCardProps extends HTMLAttributes<HTMLElement> {
  as?: SurfaceElement;
  children: ReactNode;
}

interface PageContentSurfaceProps extends HTMLAttributes<HTMLElement> {
  as?: SurfaceElement;
  children: ReactNode;
}

interface DashboardPageSurfaceProps {
  notice?: Notice | null;
  busyLabel?: string;
  actionBar?: ReactNode;
  className?: string;
  children: ReactNode;
}

interface FeatureWorkspaceLayoutProps {
  notice?: Notice | null;
  busyLabel?: string;
  actionBar?: ReactNode;
  surfaceClassName?: string;
  contentGridClassName?: string;
  summary?: ReactNode;
  workspaceEyebrow?: ReactNode;
  workspaceTitle: ReactNode;
  workspaceDescription: ReactNode;
  workspaceMeta?: ReactNode;
  workspaceContent: ReactNode;
  aside?: ReactNode;
  workspaceClassName?: string;
}

interface FlatPageLayoutProps
  extends Omit<
    FeatureWorkspaceLayoutProps,
    "surfaceClassName" | "contentGridClassName" | "workspaceClassName" | "workspaceContent"
  > {
  children: ReactNode;
  surfaceClassName?: string;
  contentGridClassName?: string;
  workspaceClassName?: string;
}

interface SidebarSectionProps {
  eyebrow?: string;
  title?: ReactNode;
  description?: ReactNode;
  className?: string;
  children?: ReactNode;
  footer?: ReactNode;
}

interface MetricCardProps {
  label: ReactNode;
  value: ReactNode;
  description?: ReactNode;
  tone?: StatusTone;
  className?: string;
}

interface KeyValueItem {
  label: ReactNode;
  value: ReactNode;
}

interface KeyValueListProps {
  items: KeyValueItem[];
  className?: string;
}

export interface EntityPickerOption {
  value: string;
  label: string;
  description?: string;
  disabled?: boolean;
}

interface EntityPickerFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: EntityPickerOption[];
  placeholder: string;
  note?: ReactNode;
  disabled?: boolean;
}

interface SettingsSelectFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: EntityPickerOption[];
  placeholder: string;
  note?: ReactNode;
  className?: string;
  style?: CSSProperties;
  labelClassName?: string;
  classNames?: SettingsSelectFieldClassNames;
  disabled?: boolean;
}

interface SettingsSelectFieldClassNames {
  root?: string;
  triggerCopy?: string;
  label?: string;
  valueGroup?: string;
  value?: string;
  chevron?: string;
  control?: string;
  note?: string;
}

interface EntityMultiPickerFieldProps {
  label: string;
  options: EntityPickerOption[];
  selectedValues: string[];
  onToggle: (value: string) => void;
  note?: ReactNode;
  disabled?: boolean;
}

interface AdvancedTextInputProps {
  label: string;
  inputLabel: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  note?: ReactNode;
  summary?: string;
  className?: string;
  disabled?: boolean;
}

export function PageHeader({
  eyebrow,
  title,
  description,
  status,
  meta,
  actions,
}: PageHeaderProps) {
  return (
    <header className="page-header">
      <div className="page-header-main">
        <div className="page-header-copy">
          {eyebrow ? <p className="page-eyebrow">{eyebrow}</p> : null}
          <div className="page-title-row">
            <h1>{title}</h1>
            {status}
          </div>
          <p className="page-description">{description}</p>
        </div>
        {actions ? <div className="page-actions">{actions}</div> : null}
      </div>
      {meta ? <div className="page-meta">{meta}</div> : null}
    </header>
  );
}

export function DashboardMetaList({
  items,
}: DashboardMetaListProps) {
  const visibleItems = getVisibleDashboardMetaItems(items);

  if (visibleItems.length === 0) {
    return null;
  }

  return (
    <>
      {visibleItems.map((item) => (
        <span className="meta-note" key={item.label}>
          {item.label}: {item.value}
        </span>
      ))}
    </>
  );
}

export function StatusBadge({
  tone = "neutral",
  children,
}: StatusBadgeProps) {
  return <span className={`status-badge status-${tone}`}>{children}</span>;
}

export function PageContentSurface({
  as: Component = "section",
  className,
  children,
  ...rest
}: PageContentSurfaceProps) {
  return (
    <Component
      className={joinClassNames("page-content-surface", className)}
      {...rest}
    >
      {children}
    </Component>
  );
}

export function DashboardPageSurface({
  notice,
  busyLabel,
  actionBar,
  className,
  children,
}: DashboardPageSurfaceProps) {
  return (
    <PageContentSurface className={className}>
      <AlertBanner notice={notice} busyLabel={busyLabel} />
      {actionBar}
      {children}
    </PageContentSurface>
  );
}

export function FeatureWorkspaceLayout({
  notice,
  busyLabel,
  actionBar,
  surfaceClassName,
  contentGridClassName,
  summary,
  workspaceEyebrow = "Workspace",
  workspaceTitle,
  workspaceDescription,
  workspaceMeta,
  workspaceContent,
  aside,
  workspaceClassName,
}: FeatureWorkspaceLayoutProps) {
  const hasWorkspaceHeader =
    workspaceEyebrow !== null ||
    workspaceTitle !== null ||
    workspaceDescription !== null ||
    workspaceMeta !== undefined;

  return (
    <DashboardPageSurface
      className={surfaceClassName}
      notice={notice}
      busyLabel={busyLabel}
      actionBar={actionBar}
    >
      {summary}

      <section
        className={joinClassNames(
          aside ? "content-grid content-grid-with-aside" : "content-grid",
          contentGridClassName,
        )}
      >
        <div className="page-main">
          <SurfaceCard
            className={joinClassNames("feature-category-panel", workspaceClassName)}
          >
            <div className="workspace-view">
              {hasWorkspaceHeader ? (
                <div className="workspace-view-header">
                  <div className="card-copy">
                    {workspaceEyebrow ? (
                      <p className="section-label">{workspaceEyebrow}</p>
                    ) : null}
                    {workspaceTitle ? <h2>{workspaceTitle}</h2> : null}
                    {workspaceDescription ? (
                      <p className="section-description">{workspaceDescription}</p>
                    ) : null}
                  </div>
                  {workspaceMeta ? (
                    <div className="workspace-view-meta">{workspaceMeta}</div>
                  ) : null}
                </div>
              ) : null}

              {workspaceContent}
            </div>
          </SurfaceCard>
        </div>

        {aside}
      </section>
    </DashboardPageSurface>
  );
}

export function FlatPageLayout({
  children,
  surfaceClassName,
  contentGridClassName,
  workspaceClassName,
  ...props
}: FlatPageLayoutProps) {
  return (
    <FeatureWorkspaceLayout
      {...props}
      surfaceClassName={joinClassNames("flat-page-surface", surfaceClassName)}
      contentGridClassName={joinClassNames(
        "flat-page-layout",
        contentGridClassName,
      )}
      workspaceClassName={joinClassNames(
        "flat-page-workspace",
        workspaceClassName,
      )}
      workspaceContent={children}
    />
  );
}

export function SurfaceCard({
  as: Component = "section",
  className,
  children,
  ...rest
}: SurfaceCardProps) {
  return (
    <Component className={joinClassNames("surface-card", className)} {...rest}>
      {children}
    </Component>
  );
}

export function SidebarSection({
  eyebrow,
  title,
  description,
  className,
  children,
  footer,
}: SidebarSectionProps) {
  return (
    <section className={joinClassNames("sidebar-section", className)}>
      {eyebrow || title || description ? (
        <div className="sidebar-section-copy">
          {eyebrow ? <p className="section-label">{eyebrow}</p> : null}
          {title ? <strong className="sidebar-section-title">{title}</strong> : null}
          {description ? (
            <p className="sidebar-section-description">{description}</p>
          ) : null}
        </div>
      ) : null}
      {children}
      {footer ? <div className="sidebar-section-footer">{footer}</div> : null}
    </section>
  );
}

export function MetricCard({
  label,
  value,
  description,
  tone = "neutral",
  className,
}: MetricCardProps) {
  return (
    <SurfaceCard
      as="article"
      className={joinClassNames(
        "metric-card",
        tone !== "neutral" && "surface-card-accent",
        tone !== "neutral" && `surface-card-accent-${tone}`,
        className,
      )}
    >
      <p className="section-label">{label}</p>
      <div className="metric-card-value-row">
        <strong className="metric-card-value">{value}</strong>
      </div>
      {description ? <p className="metric-card-description">{description}</p> : null}
    </SurfaceCard>
  );
}

export function KeyValueList({
  items,
  className,
}: KeyValueListProps) {
  const visibleItems = filterDashboardKeyValueItems(items);

  if (visibleItems.length === 0) {
    return null;
  }

  return (
    <dl className={joinClassNames("key-value-list", className)}>
      {visibleItems.map((item, index) => (
        <div className="key-value-row" key={index}>
          <dt>{item.label}</dt>
          <dd>{item.value}</dd>
        </div>
      ))}
    </dl>
  );
}

export function EntityPickerField({
  label,
  value,
  onChange,
  options,
  placeholder,
  note,
  disabled = false,
}: EntityPickerFieldProps) {
  return (
    <label className="field-stack entity-picker-field">
      <span className="field-label">{label}</span>
      <select
        aria-label={label}
        value={value}
        disabled={disabled}
        onChange={(event) => onChange(event.target.value)}
      >
        <option value="">{placeholder}</option>
        {options.map((option) => (
          <option
            key={option.value}
            value={option.value}
            disabled={option.disabled}
          >
            {option.label}
          </option>
        ))}
      </select>
      {note ? <span className="meta-note">{note}</span> : null}
    </label>
  );
}

export function SettingsSelectField({
  label,
  value,
  onChange,
  options,
  placeholder,
  note,
  className,
  style,
  labelClassName,
  classNames,
  disabled = false,
}: SettingsSelectFieldProps) {
  const selectedOption = options.find((option) => option.value === value) ?? null;
  const displayValue = selectedOption?.label ?? placeholder;
  const labelId = useId();

  return (
    <label
      className={joinClassNames(
        "settings-select-field",
        classNames?.root,
        className,
      )}
      style={style}
    >
      <span
        className={joinClassNames(
          "settings-select-trigger-copy",
          classNames?.triggerCopy,
        )}
      >
        <span
          className={joinClassNames(
            "settings-select-trigger-label",
            labelClassName,
            classNames?.label,
          )}
          id={labelId}
        >
          {label}
        </span>
        <span
          className={joinClassNames(
            "settings-select-value-group",
            classNames?.valueGroup,
          )}
        >
          <span
            className={joinClassNames(
              "settings-select-trigger-value",
              selectedOption ? null : "is-placeholder",
              classNames?.value,
            )}
            aria-hidden="true"
          >
            {displayValue}
          </span>
          <span
            className={joinClassNames("settings-select-chevron", classNames?.chevron)}
            aria-hidden="true"
          >
            <span>⌃</span>
            <span>⌄</span>
          </span>
          <select
            aria-labelledby={labelId}
            className={joinClassNames(
              "settings-select-control",
              classNames?.control,
            )}
            value={value}
            disabled={disabled}
            onChange={(event) => onChange(event.target.value)}
          >
            <option value="">{placeholder}</option>
            {options.map((option) => (
              <option
                key={option.value}
                value={option.value}
                disabled={option.disabled}
              >
                {option.label}
              </option>
            ))}
          </select>
        </span>
      </span>
      {note ? (
        <p className={joinClassNames("meta-note", classNames?.note)}>{note}</p>
      ) : null}
    </label>
  );
}

export function EntityMultiPickerField({
  label,
  options,
  selectedValues,
  onToggle,
  note,
  disabled = false,
}: EntityMultiPickerFieldProps) {
  const fieldId = useId();
  const [expanded, setExpanded] = useState(false);
  const panelId = `${fieldId}-panel`;
  const listId = `${fieldId}-list`;
  const labelId = `${fieldId}-label`;
  const valueId = `${fieldId}-value`;
  const selectedSet = new Set(
    selectedValues
      .map((value) => value.trim())
      .filter((value) => value !== ""),
  );
  const selectedOptions = options.filter((option) => selectedSet.has(option.value));
  const selectedCount = selectedSet.size;
  const summaryText =
    selectedCount === 0
      ? options.length === 0
        ? "No options available"
        : "Select one or more"
      : selectedCount === 1
        ? selectedOptions[0]?.label ?? "1 selected"
        : `${selectedCount} selected`;

  return (
    <div className="field-stack entity-multi-picker">
      <span className="field-label" id={labelId}>
        {label}
      </span>
      <button
        type="button"
        className="entity-multi-picker-trigger"
        aria-labelledby={`${labelId} ${valueId}`}
        aria-expanded={expanded}
        aria-controls={panelId}
        disabled={disabled || options.length === 0}
        onClick={() => setExpanded((current) => !current)}
      >
        <span className="entity-multi-picker-trigger-copy">
          <strong className="entity-multi-picker-trigger-value" id={valueId}>
            {summaryText}
          </strong>
          <span className="meta-note">
            {selectedCount === 0 ? "Click to choose items" : "Click to review selections"}
          </span>
        </span>
        <span className="entity-multi-picker-trigger-indicator" aria-hidden="true">
          {expanded ? "▲" : "▼"}
        </span>
      </button>
      {expanded ? (
        <div className="entity-multi-picker-panel" id={panelId}>
          <div
            className="entity-option-list"
            id={listId}
            role="group"
            aria-label={label}
          >
            {options.map((option) => {
              const checked = selectedSet.has(option.value);

              return (
                <label
                  className={joinClassNames(
                    "entity-option-card",
                    checked && "is-selected",
                    option.disabled && "is-disabled",
                  )}
                  key={option.value}
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    disabled={disabled || option.disabled}
                    onChange={() => onToggle(option.value)}
                  />
                  <span className="entity-option-copy">
                    <strong>{option.label}</strong>
                    {option.description ? (
                      <span className="meta-note">{option.description}</span>
                    ) : null}
                  </span>
                </label>
              );
            })}
          </div>
        </div>
      ) : null}
      {note ? <p className="meta-note">{note}</p> : null}
    </div>
  );
}

export function UnsavedChangesBar({
  hasUnsavedChanges,
  message = "Careful - you have unsaved changes.",
  saveLabel = "Save changes",
  resetLabel = "Reset",
  onSave,
  onReset,
  saving = false,
  disabled = false,
  className,
}: UnsavedChangesBarProps) {
  if (!hasUnsavedChanges) {
    return null;
  }

  const actionsDisabled = disabled || saving;

  return (
    <div
      className={joinClassNames("unsaved-changes-bar", className)}
      role="status"
      aria-live="polite"
      aria-busy={saving}
    >
      <div className="unsaved-changes-copy">
        <strong>{message}</strong>
      </div>
      <div className="unsaved-changes-actions">
        <button
          className="button-ghost"
          type="button"
          disabled={actionsDisabled}
          onClick={() => void onReset()}
        >
          {resetLabel}
        </button>
        <button
          className="button-primary"
          type="button"
          disabled={actionsDisabled}
          onClick={() => void onSave()}
        >
          {saveLabel}
        </button>
      </div>
    </div>
  );
}

export function AlertBanner({ notice, busyLabel }: AlertBannerProps) {
  if (!notice && !busyLabel) {
    return null;
  }

  return (
    <div
      className={`alert-banner alert-${notice?.tone ?? "info"}`}
      role="status"
    >
      <div className="card-copy">
        <strong>{notice?.message ?? busyLabel}</strong>
        {notice && busyLabel ? <p className="meta-note">{busyLabel}</p> : null}
      </div>
    </div>
  );
}

export function LookupNotice({
  title,
  message,
  retryLabel,
  onRetry,
  retryDisabled = false,
  as: Component = "div",
}: LookupNoticeProps) {
  return (
    <Component className="surface-subsection">
      <p className="section-label">{title}</p>
      <p className="meta-note">{message}</p>
      {retryLabel && onRetry ? (
        <div className="sidebar-actions">
          <button
            className="button-secondary"
            type="button"
            disabled={retryDisabled}
            onClick={() => void onRetry()}
          >
            {retryLabel}
          </button>
        </div>
      ) : null}
    </Component>
  );
}

export function EmptyState({
  title,
  description,
  action,
}: EmptyStateProps) {
  return (
    <SurfaceCard className="empty-state-card">
      <p className="section-label">Workspace</p>
      <h2>{title}</h2>
      <p>{description}</p>
      {action ? <div className="empty-state-actions">{action}</div> : null}
    </SurfaceCard>
  );
}

export function IdentityAvatar({
  imageUrl,
  label,
}: IdentityAvatarProps) {
  return (
    <div className="identity-avatar" aria-hidden="true">
      {imageUrl ? <img src={imageUrl} alt="" /> : <span>{getInitials(label)}</span>}
    </div>
  );
}

export function AdvancedTextInput({
  label,
  inputLabel,
  value,
  onChange,
  placeholder,
  note,
  summary = "Advanced",
  className,
  disabled = false,
}: AdvancedTextInputProps) {
  if (!shouldRenderDashboardDiagnosticField(label)) {
    return null;
  }

  const visibleNote = sanitizeDashboardFieldNote(label, note);

  return (
    <details className={joinClassNames("details-panel", className)}>
      <summary>{summary}</summary>
      <div className="details-content">
        <label className="field-stack">
          <span className="field-label">{label}</span>
          <input
            aria-label={inputLabel}
            disabled={disabled}
            value={value}
            onChange={(event) => onChange(event.target.value)}
            placeholder={placeholder}
          />
          {visibleNote ? <span className="meta-note">{visibleNote}</span> : null}
        </label>
      </div>
    </details>
  );
}

function joinClassNames(...parts: Array<string | null | undefined | false>) {
  return parts.filter(Boolean).join(" ");
}
