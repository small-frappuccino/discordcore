import type { ReactNode } from "react";
import { getInitials } from "../app/utils";
import type { Notice } from "../app/types";

type StatusTone = "neutral" | "info" | "success" | "error";

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
      <div className="page-header-copy">
        {eyebrow ? <p className="page-eyebrow">{eyebrow}</p> : null}
        <div className="page-title-row">
          <h1>{title}</h1>
          {status}
        </div>
        <p className="page-description">{description}</p>
        {meta ? <div className="page-meta">{meta}</div> : null}
      </div>
      {actions ? <div className="page-actions">{actions}</div> : null}
    </header>
  );
}

export function StatusBadge({
  tone = "neutral",
  children,
}: StatusBadgeProps) {
  return <span className={`status-badge status-${tone}`}>{children}</span>;
}

export function AlertBanner({ notice, busyLabel }: AlertBannerProps) {
  if (!notice && !busyLabel) {
    return null;
  }

  return (
    <div className={`alert-banner alert-${notice?.tone ?? "info"}`}>
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
    <section className="surface-card empty-state-card">
      <p className="section-label">Workspace</p>
      <h2>{title}</h2>
      <p>{description}</p>
      {action ? <div className="empty-state-actions">{action}</div> : null}
    </section>
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
