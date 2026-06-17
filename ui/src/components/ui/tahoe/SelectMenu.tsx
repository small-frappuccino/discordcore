import { useState, useRef, useEffect } from "react";

export type SelectOption = {
  value: string;
  label: string;
  disabled?: boolean;
};

type SelectMenuProps = {
  options: SelectOption[];
  value?: string;
  onChange?: (value: string) => void;
  placeholder?: string;
  className?: string;
};

export function SelectMenu({ options, value, onChange, placeholder = "Select...", className = "" }: SelectMenuProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const selectedOption = options.find((opt) => opt.value === value);

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    }
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") {
        setIsOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, []);

  return (
    <div className={`relative ${className}`} ref={containerRef}>
      <button
        type="button"
        className="tahoe-select-trigger"
        onClick={() => setIsOpen(!isOpen)}
      >
        <span>{selectedOption ? selectedOption.label : placeholder}</span>
        <div className="tahoe-select-chevron ml-auto pr-2">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <polyline points="6 15 12 9 18 15" />
          </svg>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="-mt-1">
            <polyline points="6 9 12 15 18 9" />
          </svg>
        </div>
      </button>

      <div 
        className={`tahoe-select-dropdown transition-all duration-200 ease-out origin-top-right ${
          isOpen ? 'opacity-100 scale-100 pointer-events-auto' : 'opacity-0 scale-95 pointer-events-none'
        }`}
        style={{ top: "100%", right: 0, marginTop: "4px" }}
      >
        {options.map((option) => (
          <div
            key={option.value}
            className="tahoe-select-option"
            onClick={() => {
              onChange?.(option.value);
              setIsOpen(false);
            }}
          >
            {option.label}
            {value === option.value && (
              <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="3">
                <polyline points="20 6 9 17 4 12" />
              </svg>
            )}
          </div>
        ))}
      </div>
    </div>
  );
}
