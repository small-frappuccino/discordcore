import * as React from "react";

type SliderControlProps = Omit<React.InputHTMLAttributes<HTMLInputElement>, "onChange"> & {
  value?: number;
  onChange?: (value: number) => void;
  min?: number;
  max?: number;
  minLabel?: string;
  maxLabel?: string;
};

export function SliderControl({
  value = 50,
  onChange,
  min = 0,
  max = 100,
  minLabel,
  maxLabel,
  className = "",
  ...props
}: SliderControlProps) {
  const percentage = Math.max(0, Math.min(100, ((value - min) / (max - min)) * 100));

  return (
    <div className={`flex flex-col gap-2 ${className}`}>
      <div className="flex items-center gap-3">
        {minLabel && <span className="text-xs text-muted">{minLabel}</span>}
        
        {/* We use a visually hidden native range input for accessibility, but style a custom track */}
        <div className="relative flex items-center" style={{ width: '160px', height: '16px' }}>
          <input
            type="range"
            min={min}
            max={max}
            value={value}
            onChange={(e) => onChange?.(Number(e.target.value))}
            className="absolute inset-0 w-full h-full opacity-0 cursor-pointer z-10 m-0"
            {...props}
          />
          <div className="tahoe-slider-track w-full pointer-events-none">
            <div className="tahoe-slider-fill" style={{ width: `${percentage}%` }} />
            <div className="tahoe-slider-thumb" style={{ left: `${percentage}%` }} />
          </div>
        </div>

        {maxLabel && <span className="text-xs text-muted">{maxLabel}</span>}
      </div>
    </div>
  );
}
