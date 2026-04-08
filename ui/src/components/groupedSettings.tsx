import type { HTMLAttributes, ReactNode } from "react";

type GroupedSettingsSectionElement = "div" | "section";
type GroupedSettingsLabelElement = "h2" | "h3" | "span";

interface GroupedSettingsSectionProps extends HTMLAttributes<HTMLElement> {
  as?: GroupedSettingsSectionElement;
  children: ReactNode;
}

interface GroupedSettingsStackProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
}

interface GroupedSettingsGroupProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
}

interface GroupedSettingsItemProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  stacked?: boolean;
}

interface GroupedSettingsSubrowProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
}

interface GroupedSettingsMainRowProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
}

interface GroupedSettingsCopyProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
}

interface GroupedSettingsLabelProps extends HTMLAttributes<HTMLElement> {
  as?: GroupedSettingsLabelElement;
  children: ReactNode;
}

interface GroupedSettingsSwitchClassNames {
  input?: string;
  track?: string;
  thumb?: string;
}

interface GroupedSettingsSwitchProps {
  label: string;
  checked: boolean;
  disabled?: boolean;
  onChange: (checked: boolean) => void;
  className?: string;
  classNames?: GroupedSettingsSwitchClassNames;
}

interface GroupedSettingsInlineMessageClassNames {
  copy?: string;
  action?: string;
}

interface GroupedSettingsInlineMessageProps {
  message: ReactNode;
  tone?: "info" | "error";
  action?: ReactNode;
  className?: string;
  classNames?: GroupedSettingsInlineMessageClassNames;
}

export function GroupedSettingsSection({
  as: Component = "section",
  className,
  children,
  ...props
}: GroupedSettingsSectionProps) {
  return (
    <Component className={joinClassNames("grouped-settings-section", className)} {...props}>
      {children}
    </Component>
  );
}

export function GroupedSettingsStack({
  className,
  children,
  ...props
}: GroupedSettingsStackProps) {
  return (
    <div className={joinClassNames("grouped-settings-stack", className)} {...props}>
      {children}
    </div>
  );
}

export function GroupedSettingsGroup({
  className,
  children,
  ...props
}: GroupedSettingsGroupProps) {
  return (
    <div className={joinClassNames("grouped-settings-group", className)} {...props}>
      {children}
    </div>
  );
}

export function GroupedSettingsItem({
  className,
  children,
  stacked = false,
  ...props
}: GroupedSettingsItemProps) {
  return (
    <div
      className={joinClassNames(
        "grouped-settings-item",
        stacked && "grouped-settings-item-stack",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export function GroupedSettingsSubrow({
  className,
  children,
  ...props
}: GroupedSettingsSubrowProps) {
  return (
    <div className={joinClassNames("grouped-settings-subrow", className)} {...props}>
      {children}
    </div>
  );
}

export function GroupedSettingsMainRow({
  className,
  children,
  ...props
}: GroupedSettingsMainRowProps) {
  return (
    <div className={joinClassNames("grouped-settings-main-row", className)} {...props}>
      {children}
    </div>
  );
}

export function GroupedSettingsCopy({
  className,
  children,
  ...props
}: GroupedSettingsCopyProps) {
  return (
    <div
      className={joinClassNames("card-copy", "grouped-settings-copy", className)}
      {...props}
    >
      {children}
    </div>
  );
}

export function GroupedSettingsLabel({
  as: Component = "span",
  className,
  children,
  ...props
}: GroupedSettingsLabelProps) {
  return (
    <Component className={joinClassNames("grouped-settings-label", className)} {...props}>
      {children}
    </Component>
  );
}

export function GroupedSettingsHeading({
  as: Component = "span",
  className,
  children,
  ...props
}: GroupedSettingsLabelProps) {
  return (
    <Component className={joinClassNames("grouped-settings-heading", className)} {...props}>
      {children}
    </Component>
  );
}

export function GroupedSettingsSwitch({
  label,
  checked,
  disabled = false,
  onChange,
  className,
  classNames,
}: GroupedSettingsSwitchProps) {
  return (
    <label
      className={joinClassNames(
        "grouped-settings-switch",
        checked && "is-checked",
        disabled && "is-disabled",
        className,
      )}
    >
      <input
        aria-label={label}
        checked={checked}
        disabled={disabled}
        type="checkbox"
        className={classNames?.input}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span
        className={joinClassNames("grouped-settings-switch-track", classNames?.track)}
        aria-hidden="true"
      >
        <span
          className={joinClassNames("grouped-settings-switch-thumb", classNames?.thumb)}
        />
      </span>
    </label>
  );
}

export function GroupedSettingsInlineMessage({
  message,
  tone = "error",
  action,
  className,
  classNames,
}: GroupedSettingsInlineMessageProps) {
  return (
    <div className={joinClassNames("flat-inline-message", "grouped-settings-inline-message", className)}>
      <p
        className={joinClassNames(
          "meta-note",
          "grouped-settings-inline-message-copy",
          `tone-${tone}`,
          classNames?.copy,
        )}
      >
        {message}
      </p>
      {action ? (
        <div className={joinClassNames("inline-actions", classNames?.action)}>{action}</div>
      ) : null}
    </div>
  );
}

export const GroupedSettingsBlock = GroupedSettingsSection;
export const GroupedSettingsRow = GroupedSettingsMainRow;

function joinClassNames(...parts: Array<string | null | undefined | false>) {
  return parts.filter(Boolean).join(" ");
}
