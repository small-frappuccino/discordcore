import * as React from "react";

export function SurfaceCard({
  className = "",
  children,
  ...props
}: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div className={`surface-card ${className}`} {...props}>
      {children}
    </div>
  );
}
