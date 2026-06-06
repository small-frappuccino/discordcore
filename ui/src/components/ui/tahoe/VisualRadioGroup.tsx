import * as React from "react";

type VisualRadioOption = {
  value: string;
  label: string;
  renderVisual: React.ReactNode;
};

type VisualRadioGroupProps = {
  options: VisualRadioOption[];
  value?: string;
  onChange?: (value: string) => void;
  className?: string;
};

export function VisualRadioGroup({ options, value, onChange, className = "" }: VisualRadioGroupProps) {
  return (
    <div className={`tahoe-visual-radio-group ${className}`}>
      {options.map((option) => {
        const isActive = value === option.value;
        return (
          <div
            key={option.value}
            className={`tahoe-visual-radio-item ${isActive ? "active" : ""}`}
            onClick={() => onChange?.(option.value)}
          >
            <div className="tahoe-visual-radio-box">{option.renderVisual}</div>
            <span className="tahoe-visual-radio-label">{option.label}</span>
          </div>
        );
      })}
    </div>
  );
}
