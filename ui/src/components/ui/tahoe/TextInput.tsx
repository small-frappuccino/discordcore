import * as React from "react";

export const TextInput = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  ({ className = "", ...props }, ref) => {
    return (
      <input
        ref={ref}
        className={`tahoe-text-input ${className}`}
        {...props}
      />
    );
  }
);
TextInput.displayName = "TextInput";
