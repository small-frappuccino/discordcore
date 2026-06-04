import * as React from "react";

export function SettingsGroup({
  className = "",
  children,
}: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={`settings-group ${className}`}>{children}</div>;
}
