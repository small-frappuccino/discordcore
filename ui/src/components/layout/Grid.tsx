import * as React from "react";
import { cn } from "../../lib/utils";
import { Box } from "./Box";
import type { BoxProps } from "./Box";
import type { ResponsiveProp, SpacingToken } from "../../lib/layout-utils";
import { resolveSpacing, resolveResponsiveProp } from "../../lib/layout-utils";

export type GridColumns = "1" | "2" | "3" | "4" | "5" | "6" | "7" | "8" | "9" | "10" | "11" | "12";

export interface GridProps extends BoxProps {
  columns?: ResponsiveProp<GridColumns>;
  spacing?: ResponsiveProp<SpacingToken>;
  spacingX?: ResponsiveProp<SpacingToken>;
  spacingY?: ResponsiveProp<SpacingToken>;
}

export const Grid = React.forwardRef<HTMLElement, GridProps>(
  (
    {
      as = "div",
      columns = "1",
      spacing,
      spacingX,
      spacingY,
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
          "grid",
          resolveResponsiveProp(columns, (val) => `grid-cols-${val}`),
          resolveSpacing(spacing, "gap"),
          resolveSpacing(spacingX, "gap-x"),
          resolveSpacing(spacingY, "gap-y"),
          className
        )}
        {...props}
      >
        {children}
      </Box>
    );
  }
);

Grid.displayName = "Grid";
