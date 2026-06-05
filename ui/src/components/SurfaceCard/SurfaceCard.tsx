import * as React from "react";

export const SurfaceCard = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className = "", children, ...props }, ref) => {
    return (
      <div ref={ref} className={`surface-card ${className}`} {...props}>
        {children}
      </div>
    );
  }
);
SurfaceCard.displayName = "SurfaceCard";
