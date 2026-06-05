import * as React from "react";
import { cn } from "../../../lib/utils";

export interface SurfaceCardProps extends React.HTMLAttributes<HTMLDivElement> {
  interactive?: boolean;
}

export const SurfaceCard = React.forwardRef<HTMLDivElement, SurfaceCardProps>(
  ({ className, interactive, children, ...props }, ref) => {
    return (
      <div
        ref={ref}
        className={cn(
          "surface-card",
          interactive && "cursor-pointer hover:bg-bg-surface-hover active:bg-bg-surface-active transition-colors",
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
