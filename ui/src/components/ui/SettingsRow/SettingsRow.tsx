import * as React from "react";
import { cn } from "../../../lib/utils";

type SettingsRowProps = React.HTMLAttributes<HTMLDivElement> & {
  isLast?: boolean;
};

function SettingsRowRoot({ className, isLast, children, ...props }: SettingsRowProps) {
  return (
    <div className={cn("settings-row", isLast && "is-last", className)} {...props}>
      {children}
    </div>
  );
}

function SettingsRowInfo({ className, children, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("settings-row-info", className)} {...props}>
      {children}
    </div>
  );
}

function SettingsRowTitle({ className, children, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("settings-row-title", className)} {...props}>
      {children}
    </div>
  );
}

function SettingsRowDescription({ className, children, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("settings-row-desc", className)} {...props}>
      {children}
    </div>
  );
}

function SettingsRowControl({ className, children, ...props }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={cn("settings-row-control", className)} {...props}>
      {children}
    </div>
  );
}

export const SettingsRow = Object.assign(SettingsRowRoot, {
  Info: SettingsRowInfo,
  Title: SettingsRowTitle,
  Description: SettingsRowDescription,
  Control: SettingsRowControl,
});
