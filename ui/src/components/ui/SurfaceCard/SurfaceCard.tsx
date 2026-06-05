import * as React from "react";
import { cn } from "../../../lib/utils";

import { Slot } from "../Slot/Slot";

export interface SurfaceCardProps extends React.HTMLAttributes<HTMLDivElement> {
  interactive?: boolean;
  asChild?: boolean;
}

export const SurfaceCard = React.forwardRef<HTMLDivElement, SurfaceCardProps>(
  ({ className, interactive, asChild, children, ...props }, ref) => {
          className={cn(
            "surface-card",
            interactive && "interactive",
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
          interactive && "interactive",
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
