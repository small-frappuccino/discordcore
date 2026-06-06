import * as React from "react";

export function SettingsGroup({ children, className = "" }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={`tahoe-settings-group ${className}`}>
      {children}
    </div>
  );
}
