import * as React from "react";
import { cn } from "../../../lib/utils";

import { Slot } from "../Slot/Slot";

export interface SurfaceCardProps extends React.HTMLAttributes<HTMLDivElement> {
  interactive?: boolean;
  asChild?: boolean;
}

export const SurfaceCard = React.forwardRef<HTMLDivElement, SurfaceCardProps>(
  ({ className, interactive, asChild, children, ...props }, ref) => {
    const interactiveClasses = interactive
      ? "cursor-pointer hover:bg-surface-hover hover:-translate-y-0.5 hover:shadow-md active:scale-[0.99] active:bg-surface-active transition-all duration-200 ease-out"
      : "";

    if (asChild) {
      return (
        <Slot
          ref={ref as React.ForwardedRef<HTMLElement>}
          className={cn("surface-card", interactiveClasses, className)}
          {...props}
        >
          {children}
        </Slot>
      );
    }
    return (
      <div
        ref={ref}
        className={cn("surface-card", interactiveClasses, className)}
        {...props}
      >
        {children}
      </div>
    );
  }
);

SurfaceCard.displayName = "SurfaceCard";
