import * as React from "react";
import { cn } from "../../lib/utils";

export interface StackProps extends React.HTMLAttributes<HTMLElement> {
  as?: React.ElementType;
  direction?: "vertical" | "horizontal";
  spacing?: "none" | "xs" | "sm" | "md" | "lg" | "xl" | "2xl";
  align?: "start" | "center" | "end" | "stretch";
  justify?: "start" | "center" | "end" | "between";
}

export const Stack = React.forwardRef<HTMLElement, StackProps>(
  (
    {
      as: Component = "div",
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
      <Component
        ref={ref}
        className={cn(
          "flex",
          direction === "vertical" ? "flex-col" : "flex-row",
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

Stack.displayName = "Stack";
