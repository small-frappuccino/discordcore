import * as React from "react";
import { cn } from "../../../lib/utils";

export type ToggleSwitchProps = React.InputHTMLAttributes<HTMLInputElement>;

export const ToggleSwitch = React.forwardRef<HTMLInputElement, ToggleSwitchProps>(
  ({ className, ...props }, ref) => {
    return (
      <label className={cn("relative inline-flex items-center cursor-pointer", className)}>
        <input
          type="checkbox"
          className="sr-only peer"
          ref={ref}
          {...props}
        />
        <div className="w-[40px] h-[24px] bg-[var(--bg-surface-active)] peer-focus:outline-none rounded-[12px] peer peer-checked:after:translate-x-4 peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-[10px] after:h-[20px] after:w-[20px] after:transition-all peer-checked:bg-[var(--accent-primary)]"></div>
      </label>
    );
  }
);

ToggleSwitch.displayName = "ToggleSwitch";
