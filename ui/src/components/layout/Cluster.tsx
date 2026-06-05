import * as React from "react";
import { cn } from "../../lib/utils";
import { Box } from "./Box";
import type { BoxProps } from "./Box";
import type { ResponsiveProp, SpacingToken } from "../../lib/layout-utils";
import { resolveSpacing } from "../../lib/layout-utils";

export interface ClusterProps extends BoxProps {
  spacing?: ResponsiveProp<SpacingToken>;
  align?: "start" | "center" | "end" | "stretch";
  justify?: "start" | "center" | "end" | "between";
}

export const Cluster = React.forwardRef<HTMLElement, ClusterProps>(
  (
    {
      as = "div",
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
      <Box
        as={as}
        ref={ref}
        className={cn(
          "flex flex-wrap",
          resolveSpacing(spacing, "gap"),
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
      </Box>
    );
  }
);

Cluster.displayName = "Cluster";
