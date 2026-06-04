import type { FallbackProps } from "react-error-boundary";
import { formatError } from "../../../app/utils";

export function ErrorFallback({ error, resetErrorBoundary }: FallbackProps) {
  return (
    <div
      role="alert"
      className="flex flex-col items-center justify-center min-h-[400px] p-6 text-center"
    >
      <div className="bg-destructive/10 text-destructive p-4 rounded-full mb-4">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="32"
          height="32"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z" />
          <path d="M12 9v4" />
          <path d="M12 17h.01" />
        </svg>
      </div>
      <h2 className="text-xl font-semibold mb-2">Something went wrong</h2>
      <p className="text-muted mb-6 max-w-md">
        An unexpected error occurred while rendering this page.
      </p>
      <pre className="text-sm bg-surface-base border border-border p-4 rounded-md text-left overflow-auto max-w-full mb-6">
        {formatError(error)}
      </pre>
      <button
        type="button"
        className="btn-primary"
        onClick={resetErrorBoundary}
      >
        Try again
      </button>
    </div>
  );
}
