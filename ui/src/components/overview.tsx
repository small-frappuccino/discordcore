import type { HTMLAttributes, ReactNode } from "react";
import { SurfaceCard } from "./ui";

export type OverviewTone =
  | "neutral"
  | "enabled"
  | "disabled"
  | "attention";

export type SemanticValueKind = "status" | "numeric" | "meta";

interface SectionBlockProps extends Omit<HTMLAttributes<HTMLElement>, "title"> {
  eyebrow?: ReactNode;
  title: ReactNode;
  children: ReactNode;
}

interface OverviewCardProps extends Omit<HTMLAttributes<HTMLElement>, "title"> {
  sectionLabel: ReactNode;
  title: ReactNode;
  tone?: OverviewTone;
  action?: ReactNode;
  children: ReactNode;
}

interface OverviewCardHeaderProps {
  sectionLabel: ReactNode;
  title: ReactNode;
}

interface CardActionProps {
  children: ReactNode;
}

interface OverviewStatRowProps {
  label: ReactNode;
  value: ReactNode;
  kind?: SemanticValueKind;
  tone?: OverviewTone;
  screenReaderLabel?: string;
}

interface SemanticValueProps {
  kind?: SemanticValueKind;
  tone?: OverviewTone;
  children: ReactNode;
  ariaHidden?: boolean;
}

interface StatusDotProps {
  tone: Exclude<OverviewTone, "neutral">;
}

export function SectionBlock({
  eyebrow = "Product area",
  title,
  className,
  children,
  ...rest
}: SectionBlockProps) {
  return (
    <section className={joinClassNames("overview-section-block", className)} {...rest}>
      <div className="overview-section-header">
        <div className="overview-section-copy">
          {eyebrow ? <p className="section-label">{eyebrow}</p> : null}
          <h2>{title}</h2>
        </div>
      </div>
      <div className="overview-section-body">{children}</div>
    </section>
  );
}

export function OverviewCard({
  sectionLabel,
  title,
  tone = "neutral",
  action,
  className,
  children,
  ...rest
}: OverviewCardProps) {
  return (
    <SurfaceCard
      as="article"
      className={joinClassNames(
        "overview-card",
        `overview-card-tone-${tone}`,
        className,
      )}
      {...rest}
    >
      <OverviewCardHeader sectionLabel={sectionLabel} title={title} />
      <div className="overview-card-body">{children}</div>
      {action ? <CardAction>{action}</CardAction> : null}
    </SurfaceCard>
  );
}

export function OverviewCardHeader({
  sectionLabel,
  title,
}: OverviewCardHeaderProps) {
  return (
    <div className="overview-card-header">
      <div className="overview-card-copy">
        {sectionLabel ? (
          <p className="section-label overview-card-section-label">{sectionLabel}</p>
        ) : null}
        <h3 className="overview-card-title">{title}</h3>
      </div>
    </div>
  );
}

export function OverviewStatRow({
  label,
  value,
  kind = "meta",
  tone = "neutral",
  screenReaderLabel,
}: OverviewStatRowProps) {
  return (
    <li className="overview-stat-row">
      {screenReaderLabel ? <span className="sr-only">{screenReaderLabel}</span> : null}
      <span
        className="overview-stat-label"
        aria-hidden={screenReaderLabel ? "true" : undefined}
      >
        {label}
      </span>
      <SemanticValue
        kind={kind}
        tone={tone}
        ariaHidden={screenReaderLabel !== undefined}
      >
        {value}
      </SemanticValue>
    </li>
  );
}

export function SemanticValue({
  kind = "meta",
  tone = "neutral",
  children,
  ariaHidden = false,
}: SemanticValueProps) {
  const showDot = kind !== "numeric" && tone !== "neutral";

  return (
    <span
      className={joinClassNames(
        "semantic-value",
        `semantic-value-kind-${kind}`,
        `semantic-value-tone-${tone}`,
      )}
      aria-hidden={ariaHidden ? "true" : undefined}
    >
      {showDot ? (
        <StatusDot tone={tone as Exclude<OverviewTone, "neutral">} />
      ) : null}
      <strong>{children}</strong>
    </span>
  );
}

export function StatusDot({ tone }: StatusDotProps) {
  return <span className={joinClassNames("status-dot", `status-dot-${tone}`)} aria-hidden="true" />;
}

export function CardAction({ children }: CardActionProps) {
  return <div className="overview-card-action">{children}</div>;
}

function joinClassNames(...parts: Array<string | false | null | undefined>) {
  return parts.filter(Boolean).join(" ");
}
