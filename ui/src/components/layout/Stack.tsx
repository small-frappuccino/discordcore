import * as React from "react";
import { cn } from "../../lib/utils";
import { Box, BoxProps } from "./Box";
import { ResponsiveProp, SpacingToken, resolveSpacing } from "../../lib/layout-utils";

export interface StackProps extends BoxProps {
  direction?: "vertical" | "horizontal" | { base?: "vertical" | "horizontal", sm?: "vertical" | "horizontal", md?: "vertical" | "horizontal", lg?: "vertical" | "horizontal", xl?: "vertical" | "horizontal" };
  spacing?: ResponsiveProp<SpacingToken>;
  align?: "start" | "center" | "end" | "stretch";
  justify?: "start" | "center" | "end" | "between";
}

// Since direction mapping isn't as trivial (flex-col vs flex-row), we can handle string vs object here
function resolveDirection(direction: StackProps["direction"]) {
  if (!direction) return "";
  if (typeof direction === "string") {
    return direction === "vertical" ? "flex-col" : "flex-row";
  }
  const classes: string[] = [];
  if (direction.base) classes.push(direction.base === "vertical" ? "flex-col" : "flex-row");
  if (direction.sm) classes.push(direction.sm === "vertical" ? "sm:flex-col" : "sm:flex-row");
  if (direction.md) classes.push(direction.md === "vertical" ? "md:flex-col" : "md:flex-row");
  if (direction.lg) classes.push(direction.lg === "vertical" ? "lg:flex-col" : "lg:flex-row");
  if (direction.xl) classes.push(direction.xl === "vertical" ? "xl:flex-col" : "xl:flex-row");
  return classes.join(" ");
}

export const Stack = React.forwardRef<HTMLElement, StackProps>(
  (
    {
      as = "div",
      direction = "vertical",
      spacing = "md",
      align,
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
          "flex",
          resolveDirection(direction),
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

Stack.displayName = "Stack";
