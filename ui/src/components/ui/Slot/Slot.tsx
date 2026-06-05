import * as React from "react";
import { cn } from "../../../lib/utils";

function composeRefs<T>(...refs: (React.Ref<T> | undefined)[]) {
  return (node: T) => refs.forEach(ref => {
    if (typeof ref === "function") {
      ref(node);
    } else if (ref != null) {
      (ref as React.MutableRefObject<T>).current = node;
    }
  });
}

export const Slot = React.forwardRef<HTMLElement, React.HTMLAttributes<HTMLElement>>(
  ({ children, ...props }, ref) => {
    if (React.isValidElement(children)) {
      const childProps = children.props as Record<string, unknown>;
      return React.cloneElement(children, {
        ...props,
        ...childProps,
        ref: composeRefs(ref, (children as React.ReactElement & { ref?: React.Ref<HTMLElement> }).ref),
        className: cn(props.className, childProps.className as string),
        style: { ...props.style, ...(childProps.style as React.CSSProperties) },
      } as React.HTMLAttributes<HTMLElement> & React.RefAttributes<HTMLElement>);
    }
    return React.isValidElement(children) ? children : null;
  }
);

Slot.displayName = "Slot";
