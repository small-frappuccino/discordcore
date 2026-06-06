import { useState, useRef, useEffect } from "react";
import type { SelectOption } from "./SelectMenu";

type SelectMenuMultipleProps = {
  options: SelectOption[];
  value?: string[];
  onChange?: (value: string[]) => void;
  placeholder?: string;
  className?: string;
};

export function SelectMenuMultiple({ options, value = [], onChange, placeholder = "Select...", className = "" }: SelectMenuMultipleProps) {
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const selectedOptions = options.filter((opt) => value.includes(opt.value));

  useEffect(() => {
    function handleClickOutside(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, []);

  const toggleOption = (optValue: string, e?: React.MouseEvent) => {
    if (e) {
      e.stopPropagation();
      e.preventDefault();
    }
    const newVals = value.includes(optValue)
      ? value.filter(v => v !== optValue)
      : [...value, optValue];
    onChange?.(newVals);
  };

  return (
    <div className={`relative ${className}`} ref={containerRef}>
      <div
        className="tahoe-select-multiple-trigger"
        onClick={() => setIsOpen(!isOpen)}
      >
        {selectedOptions.length === 0 ? (
          <span className="text-muted text-sm px-2">{placeholder}</span>
        ) : (
          selectedOptions.map((opt) => (
            <div key={opt.value} className="tahoe-select-token" onClick={(e) => e.stopPropagation()}>
              <span>{opt.label}</span>
              <svg 
                viewBox="0 0 24 24" 
                width="14" 
                height="14" 
                fill="none" 
                stroke="currentColor" 
                strokeWidth="2" 
                className="tahoe-select-token-remove"
                onClick={(e) => toggleOption(opt.value, e)}
              >
                <line x1="18" y1="6" x2="6" y2="18" />
                <line x1="6" y1="6" x2="18" y2="18" />
              </svg>
            </div>
          ))
        )}
        <div className="tahoe-select-chevron ml-auto pr-2">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <polyline points="6 15 12 9 18 15" />
          </svg>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="-mt-1">
            <polyline points="6 9 12 15 18 9" />
          </svg>
        </div>
      </div>

      {isOpen && (
        <div className="tahoe-select-dropdown" style={{ top: "100%", left: 0, right: 0, marginTop: "4px", maxHeight: "240px", overflowY: "auto" }}>
          {options.map((option) => {
            const isSelected = value.includes(option.value);
            return (
              <div
                key={option.value}
                className="tahoe-select-option"
                onClick={() => toggleOption(option.value)}
              >
                {option.label}
                {isSelected && (
                  <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="3">
                    <polyline points="20 6 9 17 4 12" />
                  </svg>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
