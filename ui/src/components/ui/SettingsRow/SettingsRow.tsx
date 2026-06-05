import { cn } from "../../../lib/utils";
import { Slot } from "../Slot/Slot";
import { Box } from "../../layout";
import type { BoxProps } from "../../layout";

export interface SettingsRowProps extends BoxProps {
  asChild?: boolean;
}

function SettingsRowRoot({ className, asChild, children, ...props }: SettingsRowProps) {
  if (asChild) {
    return (
      <Slot 
        className={cn("settings-row", className)} 
        {...props}
      >
        {children}
      </Slot>
    );
  }
  return (
    <Box 
      className={cn("settings-row", className)} 
      {...props}
    >
      {children}
    </Box>
  );
}

export interface SettingsRowSubComponentProps extends BoxProps {
  asChild?: boolean;
}

function SettingsRowInfo({ className, asChild, children, ...props }: SettingsRowSubComponentProps) {
  if (asChild) {
    return (
      <Slot className={cn("settings-row-info", className)} {...props}>
        {children}
      </Slot>
    );
  }
  return (
    <Box className={cn("settings-row-info", className)} {...props}>
      {children}
    </Box>
  );
}

function SettingsRowTitle({ className, asChild, children, ...props }: SettingsRowSubComponentProps) {
  if (asChild) {
    return (
      <Slot className={cn("settings-row-title text-base font-medium text-text-primary", className)} {...props}>
        {children}
      </Slot>
    );
  }
  return (
    <Box className={cn("settings-row-title text-base font-medium text-text-primary", className)} {...props}>
      {children}
    </Box>
  );
}

function SettingsRowDescription({ className, asChild, children, ...props }: SettingsRowSubComponentProps) {
  if (asChild) {
    return (
      <Slot className={cn("settings-row-desc text-sm text-text-secondary mt-1", className)} {...props}>
        {children}
      </Slot>
    );
  }
  return (
    <Box className={cn("settings-row-desc text-sm text-text-secondary mt-1", className)} {...props}>
      {children}
    </Box>
  );
}

function SettingsRowControl({ className, asChild, children, ...props }: SettingsRowSubComponentProps) {
  if (asChild) {
    return (
      <Slot className={cn("settings-row-control flex items-center", className)} {...props}>
        {children}
      </Slot>
    );
  }
  return (
    <Box className={cn("settings-row-control flex items-center", className)} {...props}>
      {children}
    </Box>
  );
}

export const SettingsRow = Object.assign(SettingsRowRoot, {
  Info: SettingsRowInfo,
  Title: SettingsRowTitle,
  Description: SettingsRowDescription,
  Control: SettingsRowControl,
});
