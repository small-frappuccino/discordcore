import * as React from "react";
import { cn } from "../../../lib/utils";
import { Slot } from "../Slot/Slot";

export interface FormControlProps extends React.HTMLAttributes<HTMLDivElement> {
  asChild?: boolean;
}

export const FormControl = React.forwardRef<HTMLDivElement, FormControlProps>(
  ({ className, asChild, children, ...props }, ref) => {
    if (asChild) {
      return (
        <Slot
          ref={ref as React.ForwardedRef<HTMLElement>}
          className={cn("w-full max-w-sm", className)}
          {...props}
        >
          {children}
        </Slot>
      );
    }
    return (
      <div
        ref={ref}
        className={cn("w-full max-w-sm", className)}
        {...props}
      >
        {children}
      </div>
    );
  }
);

FormControl.displayName = "FormControl";
