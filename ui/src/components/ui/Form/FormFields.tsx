import * as React from "react";
import { useFormContext, type RegisterOptions } from "react-hook-form";
import { cn } from "../../../lib/utils";
import { Select } from "../Select/Select";

// Helper to resolve nested errors, e.g. "automation.transcriptChannelId"
function getError(obj: Record<string, unknown>, path: string) {
  if (!obj || !path) return undefined;
  const keys = path.split('.');
  let current: Record<string, unknown> = obj;
  for (const key of keys) {
    if (current[key] === undefined) return undefined;
    current = current[key] as Record<string, unknown>;
  }
  return current;
}

export interface FormInputProps extends React.InputHTMLAttributes<HTMLInputElement> {
  name: string;
  rules?: RegisterOptions;
}

export function FormInput({ name, rules, className, ...props }: FormInputProps) {
  const { register, formState: { errors } } = useFormContext();
  const error = getError(errors as Record<string, unknown>, name);

  return (
    <div className="flex flex-col w-full">
      <input
        {...register(name, rules)}
        className={cn("form-input transition-all duration-200 ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-1 focus-visible:ring-offset-base disabled:opacity-70 disabled:cursor-not-allowed hover:bg-white/5", className)}
        {...props}
      />
      {error && (
        <p className="text-red-500 text-xs mt-1">
          {error.message as string}
        </p>
      )}
    </div>
  );
}

export interface FormSelectProps extends React.SelectHTMLAttributes<HTMLSelectElement> {
  name: string;
  rules?: RegisterOptions;
}

export function FormSelect({ name, rules, className, children, ...props }: FormSelectProps) {
  const { register, formState: { errors } } = useFormContext();
  const error = getError(errors as Record<string, unknown>, name);

  return (
    <div className="flex flex-col w-full">
      <Select
        {...register(name, rules)}
        className={className}
        {...props}
      >
        {children}
      </Select>
      {error && (
        <p className="text-red-500 text-xs mt-1">
          {error.message as string}
        </p>
      )}
    </div>
  );
}

export interface FormCheckboxProps extends React.InputHTMLAttributes<HTMLInputElement> {
  name: string;
  rules?: RegisterOptions;
}

export function FormCheckbox({ name, rules, className, ...props }: FormCheckboxProps) {
  const { register, formState: { errors } } = useFormContext();
  const error = getError(errors as Record<string, unknown>, name);

  return (
    <div className="flex flex-col items-start">
      <input
        type="checkbox"
        {...register(name, rules)}
        className={cn("form-checkbox transition-all duration-200 ease-out focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-500 focus-visible:ring-offset-1 focus-visible:ring-offset-base disabled:opacity-70 disabled:cursor-not-allowed cursor-pointer", className)}
        {...props}
      />
      {error && (
        <p className="text-red-500 text-xs mt-1">
          {error.message as string}
        </p>
      )}
    </div>
  );
}
