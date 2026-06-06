

type SegmentedControlOption = {
  value: string;
  label: string;
};

type SegmentedControlProps = {
  options: SegmentedControlOption[];
  value?: string;
  onChange?: (value: string) => void;
  className?: string;
};

export function SegmentedControl({ options, value, onChange, className = "" }: SegmentedControlProps) {
  const activeIndex = Math.max(
    0,
    options.findIndex((opt) => opt.value === value)
  );

  return (
    <div className={`tahoe-segmented-control ${className}`}>
      <div
        className="tahoe-segmented-pill"
        style={{
          width: `${100 / options.length}%`,
          transform: `translateX(${activeIndex * 100}%)`,
        }}
      />
      {options.map((option) => {
        const isActive = value === option.value;
        return (
          <div
            key={option.value}
            className={`tahoe-segmented-item ${isActive ? "active" : ""}`}
            onClick={() => onChange?.(option.value)}
          >
            {option.label}
          </div>
        );
      })}
    </div>
  );
}
