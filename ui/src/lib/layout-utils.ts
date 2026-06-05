export type SpacingToken = "none" | "xs" | "sm" | "md" | "lg" | "xl" | "2xl";

export type ResponsiveProp<T> =
  | T
  | {
      base?: T;
      sm?: T;
      md?: T;
      lg?: T;
      xl?: T;
      "2xl"?: T;
    };

const spacingMap: Record<SpacingToken, string> = {
  none: "0",
  xs: "1",
  sm: "2",
  md: "4",
  lg: "6",
  xl: "8",
  "2xl": "12",
};

type SpacingPrefix = 
  | "gap" | "gap-x" | "gap-y"
  | "p" | "px" | "py" | "pt" | "pb" | "pl" | "pr"
  | "m" | "mx" | "my" | "mt" | "mb" | "ml" | "mr";

export function resolveResponsiveProp<T extends string>(
  prop: ResponsiveProp<T> | undefined,
  resolver: (value: T) => string
): string {
  if (!prop) return "";

  if (typeof prop === "string") {
    return resolver(prop as T);
  }

  const classes: string[] = [];
  if (prop.base) classes.push(resolver(prop.base));
  if (prop.sm) classes.push(`sm:${resolver(prop.sm)}`);
  if (prop.md) classes.push(`md:${resolver(prop.md)}`);
  if (prop.lg) classes.push(`lg:${resolver(prop.lg)}`);
  if (prop.xl) classes.push(`xl:${resolver(prop.xl)}`);
  if (prop["2xl"]) classes.push(`2xl:${resolver(prop["2xl"])}`);

  return classes.join(" ");
}

export function resolveSpacing(
  prop: ResponsiveProp<SpacingToken> | undefined,
  prefix: SpacingPrefix
): string {
  return resolveResponsiveProp(prop, (val) => `${prefix}-${spacingMap[val]}`);
}
