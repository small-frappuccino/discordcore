import * as React from "react";

// --- Buttons ---

type ButtonProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "danger" | "ghost";
  size?: "sm" | "md" | "lg";
};

export function Button({
  className = "",
  variant = "primary",
  size = "md",
  ...props
}: ButtonProps) {
  return (
    <button
      className={`btn btn-${variant} btn-${size} ${className}`}
      {...props}
    />
  );
}

// --- Cards ---

export function SurfaceCard({
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={`surface-card ${className}`} {...props}>
      {children}
    </div>
  );
}

// --- Badges ---

type BadgeProps = React.HTMLAttributes<HTMLSpanElement> & {
  variant?: "success" | "danger" | "warning" | "neutral";
};

export function Badge({
  className = "",
  variant = "neutral",
  children,
  ...props
}: BadgeProps) {
  return (
    <span className={`badge badge-${variant} ${className}`} {...props}>
      {children}
    </span>
  );
}

// --- Page Header ---

type PageHeaderProps = {
  title: string;
  description?: string;
  badge?: React.ReactNode;
};

export function PageHeader({ title, description, badge }: PageHeaderProps) {
  return (
    <div className="page-header">
      <div className="page-header-title-row">
        <h1 className="page-title">{title}</h1>
        {badge}
      </div>
      {description && <p className="page-description">{description}</p>}
    </div>
  );
}

// --- List Items / Settings Rows ---

export function SettingsGroup({
  className = "",
  children,
}: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={`settings-group ${className}`}>{children}</div>;
}

type SettingsRowProps = {
  title: string;
  description?: string;
  control?: React.ReactNode;
  isLast?: boolean;
};

export function SettingsRow({ title, description, control, isLast }: SettingsRowProps) {
  return (
    <div className={`settings-row ${isLast ? "is-last" : ""}`}>
      <div className="settings-row-info">
        <div className="settings-row-title">{title}</div>
        {description && <div className="settings-row-desc">{description}</div>}
      </div>
      {control && <div className="settings-row-control">{control}</div>}
    </div>
  );
}
