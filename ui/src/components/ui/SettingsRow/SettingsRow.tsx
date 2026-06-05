import * as React from "react";
import { cn } from "../../../lib/utils";
import { motion } from "framer-motion";
import { Slot } from "../Slot/Slot";

export interface SettingsRowRootProps extends React.ComponentProps<typeof motion.div> {
  asChild?: boolean;
}

const MotionSlot = motion.create(Slot);

function SettingsRowRoot({ className, asChild, children, ...props }: SettingsRowRootProps) {
  if (asChild) {
    return (
      <MotionSlot 
        initial={{ opacity: 0, y: -10 }}
        animate={{ opacity: 1, y: 0 }}
        className={cn("settings-row", className)} 
        {...props}
      >
        {children}
      </MotionSlot>
    );
  }
  return (
    <motion.div 
      initial={{ opacity: 0, y: -10 }}
      animate={{ opacity: 1, y: 0 }}
      className={cn("settings-row", className)} 
      {...props}
    >
      {children}
    </motion.div>
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
