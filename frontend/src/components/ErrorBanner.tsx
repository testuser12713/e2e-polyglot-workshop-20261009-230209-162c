import { ApiError } from "../lib/api";

export type AlertVariant = "info" | "success" | "warning" | "error";

export interface ErrorBannerProps {
  error?: ApiError | { code?: string; message: string } | string | null;
  variant?: AlertVariant;
  onClose?: () => void;
}

function normalize(
  error: ErrorBannerProps["error"],
): { code: string | null; message: string } | null {
  if (!error) {
    return null;
  }
  if (typeof error === "string") {
    return { code: null, message: error };
  }
  return {
    code: error.code ?? null,
    message: error.message || "Unbekannter Fehler",
  };
}

export default function ErrorBanner({ error, variant = "error", onClose }: ErrorBannerProps) {
  const normalized = normalize(error);
  if (!normalized) {
    return null;
  }
  const role = variant === "error" || variant === "warning" ? "alert" : "status";

  return (
    <div
      className={`alert alert--${variant}`}
      role={role}
      data-error-code={normalized.code ?? undefined}
    >
      <span className="alert__icon" aria-hidden="true">
        {variant === "success" ? "\u2713" : variant === "info" ? "\u2139" : "\u26A0"}
      </span>
      <p className="alert__message">{normalized.message}</p>
      {onClose ? (
        <button
          type="button"
          className="alert__close"
          onClick={onClose}
          aria-label="Meldung schließen"
        >
          {"\u00D7"}
        </button>
      ) : null}
    </div>
  );
}
