import * as React from "react";
import { cn } from "../../lib/utils";

export interface ClusterProps extends React.HTMLAttributes<HTMLElement> {
  as?: React.ElementType;
  spacing?: "none" | "xs" | "sm" | "md" | "lg" | "xl" | "2xl";
  align?: "start" | "center" | "end" | "stretch";
  justify?: "start" | "center" | "end" | "between";
}

export const Cluster = React.forwardRef<HTMLElement, ClusterProps>(
  (
    {
      as: Component = "div",
      spacing = "md",
      align = "center",
      justify,
      className,
      children,
      ...props
    },
    ref
  ) => {
    return (
      <Component
        ref={ref}
        className={cn(
          "flex flex-wrap",
          {
            "gap-0": spacing === "none",
            "gap-1": spacing === "xs",
            "gap-2": spacing === "sm",
            "gap-4": spacing === "md",
            "gap-6": spacing === "lg",
            "gap-8": spacing === "xl",
            "gap-12": spacing === "2xl",
          },
          align === "start" && "items-start",
          align === "center" && "items-center",
          align === "end" && "items-end",
          align === "stretch" && "items-stretch",
          justify === "start" && "justify-start",
          justify === "center" && "justify-center",
          justify === "end" && "justify-end",
          justify === "between" && "justify-between",
          className
        )}
        {...props}
      >
        {children}
      </Component>
    );
  }
);

Cluster.displayName = "Cluster";
