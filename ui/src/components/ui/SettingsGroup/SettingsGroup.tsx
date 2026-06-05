import * as React from "react";
import { cn } from "../../../lib/utils";

export const SettingsGroup = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, children, ...props }, ref) => {
    return (
      <div ref={ref} className={cn("settings-group animate-in fade-in slide-in-from-bottom-2 duration-300 ease-out", className)} {...props}>
        {children}
      </div>
    );
  }
);

SettingsGroup.displayName = "SettingsGroup";
