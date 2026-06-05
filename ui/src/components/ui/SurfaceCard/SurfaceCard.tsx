import * as React from "react";
import { cn } from "../../../lib/utils";

import { Slot } from "../Slot/Slot";

export interface SurfaceCardProps extends React.HTMLAttributes<HTMLDivElement> {
  interactive?: boolean;
  asChild?: boolean;
}

export const SurfaceCard = React.forwardRef<HTMLDivElement, SurfaceCardProps>(
  ({ className, interactive, asChild, children, ...props }, ref) => {
    if (asChild) {
      return (
        <Slot
          ref={ref as React.ForwardedRef<HTMLElement>}
          className={cn(
            "surface-card",
            interactive && "cursor-pointer hover:bg-[var(--bg-surface-hover)] active:bg-[var(--bg-surface-active)] transition-colors",
            className
          )}
          {...props}
        >
          {children}
        </Slot>
      );
    }
    return (
      <div
        ref={ref}
        className={cn(
          "surface-card",
          interactive && "cursor-pointer hover:bg-surface-hover active:bg-surface-active transition-colors",
          className
        )}
        {...props}
      >
        {children}
      </div>
    );
  }
);

SurfaceCard.displayName = "SurfaceCard";
