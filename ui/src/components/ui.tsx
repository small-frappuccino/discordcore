import type { HTMLAttributes, ReactNode } from "react";
import { getInitials } from "../app/utils";
import type { Notice } from "../app/types";

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

interface StatusBadgeProps {
  tone?: StatusTone;
  children: ReactNode;
}

interface SurfaceCardProps extends HTMLAttributes<HTMLElement> {
  as?: SurfaceElement;
  children: ReactNode;
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

interface EntityMultiPickerFieldProps {
  label: string;
  options: EntityPickerOption[];
  selectedValues: string[];
  onToggle: (value: string) => void;
  note?: ReactNode;
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

export function StatusBadge({
  tone = "neutral",
  children,
}: StatusBadgeProps) {
  return <span className={`status-badge status-${tone}`}>{children}</span>;
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
    <SurfaceCard as="article" className={joinClassNames("metric-card", className)}>
      <p className="section-label">{label}</p>
      <div className="metric-card-value-row">
        <strong className="metric-card-value">{value}</strong>
        {tone !== "neutral" ? <span className={`metric-card-dot tone-${tone}`} /> : null}
      </div>
      {description ? <p className="metric-card-description">{description}</p> : null}
    </SurfaceCard>
  );
}

export function KeyValueList({
  items,
  className,
}: KeyValueListProps) {
  return (
    <dl className={joinClassNames("key-value-list", className)}>
      {items.map((item, index) => (
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

export function EntityMultiPickerField({
  label,
  options,
  selectedValues,
  onToggle,
  note,
  disabled = false,
}: EntityMultiPickerFieldProps) {
  const selectedSet = new Set(
    selectedValues
      .map((value) => value.trim())
      .filter((value) => value !== ""),
  );

  return (
    <div className="entity-multi-picker">
      <p className="section-label">{label}</p>
      <div className="entity-option-list" role="group" aria-label={label}>
        {options.map((option) => {
          const checked = selectedSet.has(option.value);

          return (
            <label className="entity-option-card" key={option.value}>
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
      {note ? <p className="meta-note">{note}</p> : null}
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
      <div>
        <p className="section-label">Workspace status</p>
        <strong>{notice?.message ?? busyLabel}</strong>
      </div>
      {busyLabel ? <span className="meta-pill subtle-pill">{busyLabel}</span> : null}
    </div>
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

function joinClassNames(...parts: Array<string | null | undefined | false>) {
  return parts.filter(Boolean).join(" ");
}
