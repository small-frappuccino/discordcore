import * as React from "react";

type PageContainerProps = React.HTMLAttributes<HTMLDivElement> & {
  children: React.ReactNode;
};

export function PageContainer({ className = "", children, ...props }: PageContainerProps) {
  return (
    <div className={`flex flex-col h-full w-full max-w-7xl mx-auto px-4 ${className}`} {...props}>
      {children}
    </div>
  );
}
