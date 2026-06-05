import * as React from "react";
import { cn } from "../../../lib/utils";
import { Stack, StackProps } from "../../layout";

export const SettingsGroup = React.forwardRef<HTMLElement, StackProps>(
  ({ className, children, spacing = "md", ...props }, ref) => {
    return (
      <Stack ref={ref} className={cn("settings-group", className)} spacing={spacing} {...props}>
        {children}
      </Stack>
    );
  }
);

SettingsGroup.displayName = "SettingsGroup";
