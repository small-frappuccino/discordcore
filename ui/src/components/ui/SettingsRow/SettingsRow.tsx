import * as React from "react";

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
