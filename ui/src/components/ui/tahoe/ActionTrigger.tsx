import * as React from "react";

type ActionTriggerProps = React.ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: "primary" | "secondary" | "danger";
  isLoading?: boolean;
};

export const ActionTrigger = React.forwardRef<HTMLButtonElement, ActionTriggerProps>(
  ({ className = "", variant = "secondary", isLoading, children, disabled, ...props }, ref) => {
    let variantClass = "";
    if (variant === "primary") variantClass = "tahoe-action-trigger--primary";
    if (variant === "danger") variantClass = "tahoe-action-trigger--danger";

    return (
      <button
        ref={ref}
        type="button"
        className={`tahoe-action-trigger ${variantClass} ${className}`}
        disabled={disabled || isLoading}
        {...props}
      >
        {isLoading ? "Loading..." : children}
      </button>
    );
  }
);
ActionTrigger.displayName = "ActionTrigger";
