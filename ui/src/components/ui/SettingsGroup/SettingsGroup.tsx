import * as React from "react";
import { cn } from "../../../lib/utils";

export const SettingsGroup = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, children, ...props }, ref) => {
    return (
      <div ref={ref} className={cn("settings-group", className)} {...props}>
        {children}
      </div>
    );
  }
);

SettingsGroup.displayName = "SettingsGroup";
