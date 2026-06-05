import * as React from "react";
import { cn } from "../../../lib/utils";
import { Stack } from "../../layout";
import type { StackProps } from "../../layout";

export const SettingsGroup = React.forwardRef<HTMLElement, StackProps>(
  ({ className, children, spacing = "md", ...props }, ref) => {
    return (
      <Stack ref={ref} className={cn("settings-group animate-in fade-in slide-in-from-bottom-2 duration-300 ease-out", className)} spacing={spacing} {...props}>
        {children}
      </Stack>
    );
  }
);

SettingsGroup.displayName = "SettingsGroup";
