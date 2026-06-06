import * as React from "react";

type SettingsRowProps = {
  title: React.ReactNode;
  description?: React.ReactNode;
  control?: React.ReactNode;
  interactive?: boolean;
  onClick?: () => void;
  href?: string;
  className?: string;
  isMultiline?: boolean;
};

export function SettingsRow({
  title,
  description,
  control,
  interactive,
  onClick,
  href,
  className = "",
  isMultiline,
}: SettingsRowProps) {
  const isInteractive = interactive || !!onClick || !!href;
  const classes = [
    "tahoe-settings-row",
    isMultiline ? "tahoe-settings-row--multiline" : "",
    isInteractive ? "tahoe-settings-row--interactive" : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");

  const content = (
    <>
      <div className="tahoe-settings-row-info">
        <div className="tahoe-settings-row-title">{title}</div>
        {description && <div className="tahoe-settings-row-desc">{description}</div>}
      </div>
      {control && <div className="tahoe-settings-row-control">{control}</div>}
    </>
  );

  if (href) {
    return (
      <a href={href} className={classes} onClick={onClick}>
        {content}
      </a>
    );
  }

  if (isInteractive) {
    return (
      <button type="button" className={classes} onClick={onClick}>
        {content}
      </button>
    );
  }

  return <div className={classes}>{content}</div>;
}
