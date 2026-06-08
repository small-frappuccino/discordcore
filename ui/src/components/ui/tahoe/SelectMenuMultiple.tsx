import { useState, useRef, useEffect, useMemo } from "react";
import type { SelectOption } from "./SelectMenu";
import { useVirtualizer } from '@tanstack/react-virtual';

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
  const dropdownRef = useRef<HTMLDivElement>(null);

  const selectedOptionsSet = useMemo(() => new Set(value), [value]);
  const selectedOptions = useMemo(() => options.filter((opt) => selectedOptionsSet.has(opt.value)), [options, selectedOptionsSet]);

  const rowVirtualizer = useVirtualizer({
    count: options.length,
    getScrollElement: () => dropdownRef.current,
    estimateSize: () => 35,
    overscan: 5,
  });

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
    const option = options.find(o => o.value === optValue);
    if (option?.disabled) return;
    
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
        <div 
          className="tahoe-select-dropdown" 
          ref={dropdownRef}
          style={{ top: "100%", left: 0, right: 0, marginTop: "4px", maxHeight: "240px", overflowY: "auto" }}
        >
          <div style={{ height: `${rowVirtualizer.getTotalSize()}px`, width: '100%', position: 'relative' }}>
            {rowVirtualizer.getVirtualItems().map((virtualRow) => {
              const option = options[virtualRow.index];
              const isSelected = selectedOptionsSet.has(option.value);
              return (
                <div
                  key={virtualRow.key}
                  className={`tahoe-select-option ${option.disabled ? 'opacity-50 cursor-not-allowed' : ''}`}
                  style={{
                    position: 'absolute',
                    top: 0,
                    left: 0,
                    width: '100%',
                    height: `${virtualRow.size}px`,
                    transform: `translateY(${virtualRow.start}px)`,
                  }}
                  onClick={() => !option.disabled && toggleOption(option.value)}
                >
                  {option.label}
                  {option.disabled && <span className="ml-auto text-xs text-text-muted">Missing Perms</span>}
                  {isSelected && !option.disabled && (
                    <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="3">
                      <polyline points="20 6 9 17 4 12" />
                    </svg>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
