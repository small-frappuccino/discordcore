import * as React from "react";
import { cn } from "../../lib/utils";
import type { ResponsiveProp, SpacingToken } from "../../lib/layout-utils";
import { resolveSpacing } from "../../lib/layout-utils";

export interface BoxProps extends React.HTMLAttributes<HTMLElement> {
  as?: React.ElementType;
  m?: ResponsiveProp<SpacingToken>;
  mx?: ResponsiveProp<SpacingToken>;
  my?: ResponsiveProp<SpacingToken>;
  mt?: ResponsiveProp<SpacingToken>;
  mb?: ResponsiveProp<SpacingToken>;
  ml?: ResponsiveProp<SpacingToken>;
  mr?: ResponsiveProp<SpacingToken>;
  p?: ResponsiveProp<SpacingToken>;
  px?: ResponsiveProp<SpacingToken>;
  py?: ResponsiveProp<SpacingToken>;
  pt?: ResponsiveProp<SpacingToken>;
  pb?: ResponsiveProp<SpacingToken>;
  pl?: ResponsiveProp<SpacingToken>;
  pr?: ResponsiveProp<SpacingToken>;
}

export const Box = React.forwardRef<HTMLElement, BoxProps>(
  (
    {
      as: Component = "div",
      className,
      m, mx, my, mt, mb, ml, mr,
      p, px, py, pt, pb, pl, pr,
      children,
      ...props
    },
    ref
  ) => {
    return (
      <Component
        ref={ref}
        className={cn(
          resolveSpacing(m, "m"),
          resolveSpacing(mx, "mx"),
          resolveSpacing(my, "my"),
          resolveSpacing(mt, "mt"),
          resolveSpacing(mb, "mb"),
          resolveSpacing(ml, "ml"),
          resolveSpacing(mr, "mr"),
          resolveSpacing(p, "p"),
          resolveSpacing(px, "px"),
          resolveSpacing(py, "py"),
          resolveSpacing(pt, "pt"),
          resolveSpacing(pb, "pb"),
          resolveSpacing(pl, "pl"),
          resolveSpacing(pr, "pr"),
          className
        )}
        {...props}
      >
        {children}
      </Component>
    );
  }
);

Box.displayName = "Box";
