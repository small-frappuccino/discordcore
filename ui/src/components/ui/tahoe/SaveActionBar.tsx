import { Button } from "..";

interface SaveActionBarProps {
  isDirty: boolean;
  isSaving: boolean;
  onSave: () => void;
  onReset: () => void;
  saveError?: string | null;
  onClearError?: () => void;
}

export function SaveActionBar({
  isDirty,
  isSaving,
  onSave,
  onReset,
  saveError,
  onClearError
}: SaveActionBarProps) {
  const isVisible = isDirty || saveError;

  return (
    <div 
      className={`fixed bottom-0 left-0 right-0 p-4 md:p-6 pointer-events-none z-40 transition-all duration-300 ease-in-out transform ${
        isVisible ? "translate-y-0 opacity-100" : "translate-y-[120%] opacity-0"
      }`}
    >
      <div className="max-w-4xl mx-auto pointer-events-auto">
        <div className="bg-[#111214] border border-[var(--border-subtle)] shadow-2xl rounded-xl p-3 md:p-4 flex flex-col md:flex-row items-center justify-between gap-4">
          <div className="flex-1 min-w-0">
            {saveError ? (
              <div className="flex items-center gap-2 text-[var(--danger)]">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="12" cy="12" r="10"></circle>
                  <line x1="12" y1="8" x2="12" y2="12"></line>
                  <line x1="12" y1="16" x2="12.01" y2="16"></line>
                </svg>
                <span className="text-sm font-medium truncate">{saveError}</span>
              </div>
            ) : (
              <p className="text-[var(--text-primary)] font-medium text-sm md:text-base">
                Careful — you have unsaved changes!
              </p>
            )}
          </div>
          
          <div className="flex items-center gap-3 shrink-0">
            <Button 
              variant="ghost" 
              onClick={() => {
                if (saveError && onClearError) onClearError();
                else onReset();
              }}
              disabled={isSaving}
              className="text-[var(--text-primary)] hover:underline transition-colors"
            >
              {saveError ? "Dismiss" : "Reset"}
            </Button>
            <Button 
              variant="primary" 
              onClick={onSave} 
              isLoading={isSaving}
              disabled={isSaving}
              className="min-w-[120px] transition-all"
            >
              Save Changes
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
