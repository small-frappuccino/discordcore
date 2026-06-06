import * as React from "react";

export const TextArea = React.forwardRef<HTMLTextAreaElement, React.TextareaHTMLAttributes<HTMLTextAreaElement>>(
  ({ className = "", ...props }, ref) => {
    return (
      <textarea
        ref={ref}
        className={`tahoe-text-input tahoe-text-area ${className}`}
        {...props}
      />
    );
  }
);
TextArea.displayName = "TextArea";
