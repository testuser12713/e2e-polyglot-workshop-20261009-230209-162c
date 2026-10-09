import { useId, type ChangeEvent } from "react";

export interface FormFieldProps {
  label: string;
  name: string;
  value: string;
  onChange: (event: ChangeEvent<HTMLInputElement>) => void;
  onBlur?: () => void;
  type?: string;
  error?: string | null;
  touched?: boolean;
  required?: boolean;
  disabled?: boolean;
  readOnly?: boolean;
  placeholder?: string;
  hint?: string;
  autoComplete?: string;
  inputMode?: "text" | "numeric" | "decimal" | "email" | "tel";
  maxLength?: number;
}

export default function FormField({
  label,
  name,
  value,
  onChange,
  onBlur,
  type = "text",
  error,
  touched = false,
  required = false,
  disabled = false,
  readOnly = false,
  placeholder,
  hint,
  autoComplete,
  inputMode,
  maxLength,
}: FormFieldProps) {
  const generatedId = useId();
  const inputId = `${name}-${generatedId}`;
  const errorId = `${inputId}-error`;
  const hintId = `${inputId}-hint`;
  const showError = Boolean(touched) && Boolean(error);
  const describedBy = [showError ? errorId : null, hint ? hintId : null]
    .filter(Boolean)
    .join(" ");

  return (
    <div className={`field${showError ? " field--error" : ""}`}>
      <label className="field__label" htmlFor={inputId}>
        {label}
        {required ? <span className="field__required"> *</span> : null}
      </label>
      <input
        id={inputId}
        className="field__input"
        name={name}
        type={type}
        value={value}
        onChange={onChange}
        onBlur={onBlur}
        disabled={disabled}
        readOnly={readOnly}
        placeholder={placeholder}
        autoComplete={autoComplete}
        inputMode={inputMode}
        maxLength={maxLength}
        aria-invalid={showError || undefined}
        aria-describedby={describedBy || undefined}
        aria-required={required || undefined}
      />
      {hint ? (
        <p className="field__hint" id={hintId}>
          {hint}
        </p>
      ) : null}
      {showError ? (
        <p className="field__error" id={errorId} role="alert">
          {error}
        </p>
      ) : null}
    </div>
  );
}
