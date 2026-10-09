import { useRef, useState, type ChangeEvent, type FormEvent } from "react";
import { useLocation, useNavigate } from "react-router-dom";
import ErrorBanner from "../components/ErrorBanner";
import FormField from "../components/FormField";
import { ApiError } from "../lib/api";
import { useAuth } from "../lib/auth";

const GENERIC_LOGIN_ERROR = "E-Mail oder Passwort ist falsch.";
const RATE_LIMIT_MESSAGE = "Zu viele Anmeldeversuche. Bitte warten Sie eine Minute.";
const EMAIL_ERROR = "Bitte geben Sie eine gültige E-Mail-Adresse ein.";
const PASSWORD_ERROR = "Bitte geben Sie Ihr Passwort ein.";
const VALIDATION_SUMMARY = "Bitte prüfen Sie die markierten Felder.";
const GENERIC_FALLBACK_ERROR =
  "Die Anmeldung ist fehlgeschlagen. Bitte versuchen Sie es erneut.";

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;

interface FormErrors {
  email: string | null;
  password: string | null;
}

interface ServerMessage {
  code: string | null;
  message: string;
  variant: "error" | "warning";
}

function validate(email: string, password: string): FormErrors {
  return {
    email: EMAIL_PATTERN.test(email.trim()) ? null : EMAIL_ERROR,
    password: password ? null : PASSWORD_ERROR,
  };
}

export default function LoginPage() {
  const { login } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const formRef = useRef<HTMLFormElement>(null);

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [touched, setTouched] = useState({ email: false, password: false });
  const [submitAttempted, setSubmitAttempted] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [serverMessage, setServerMessage] = useState<ServerMessage | null>(null);

  const errors = validate(email, password);
  const redirectState = location.state as { from?: string } | null;
  const redirectTo = redirectState?.from || "/werkstatt";

  const showFieldError = (field: "email" | "password") => touched[field] || submitAttempted;

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitAttempted(true);
    setServerMessage(null);

    const current = validate(email, password);
    if (current.email || current.password) {
      const firstInvalid = current.email ? "email" : "password";
      formRef.current?.querySelector<HTMLInputElement>(`[name="${firstInvalid}"]`)?.focus();
      return;
    }

    setSubmitting(true);
    try {
      await login(email.trim(), password);
      navigate(redirectTo, { replace: true });
    } catch (error) {
      if (error instanceof ApiError && error.status === 429) {
        setServerMessage({ code: error.code, message: RATE_LIMIT_MESSAGE, variant: "warning" });
      } else if (error instanceof ApiError && error.status === 401) {
        setServerMessage({ code: error.code, message: GENERIC_LOGIN_ERROR, variant: "error" });
      } else if (error instanceof ApiError) {
        setServerMessage({ code: error.code, message: error.message, variant: "error" });
      } else {
        setServerMessage({ code: null, message: GENERIC_FALLBACK_ERROR, variant: "error" });
      }
    } finally {
      setSubmitting(false);
    }
  }

  const summary = submitAttempted && (errors.email || errors.password) ? VALIDATION_SUMMARY : null;

  return (
    <section className="page page--narrow">
      <header className="page__header">
        <h1 className="page__title">Werkstatt-Anmeldung</h1>
        <p className="page__subline">Melden Sie sich an, um den Werkstattbereich zu nutzen.</p>
      </header>
      <div className="card">
        <form ref={formRef} onSubmit={handleSubmit} noValidate>
          {summary ? <ErrorBanner error={{ message: summary }} variant="error" /> : null}
          {serverMessage ? (
            <ErrorBanner
              error={{ code: serverMessage.code ?? undefined, message: serverMessage.message }}
              variant={serverMessage.variant}
            />
          ) : null}

          <div style={{ marginTop: summary || serverMessage ? "var(--space-3)" : 0 }}>
            <FormField
              label="E-Mail"
              name="email"
              type="email"
              value={email}
              onChange={(event: ChangeEvent<HTMLInputElement>) => setEmail(event.target.value)}
              onBlur={() => setTouched((state) => ({ ...state, email: true }))}
              error={errors.email}
              touched={showFieldError("email")}
              autoComplete="email"
              inputMode="email"
              placeholder="name@werkstatt.de"
            />
          </div>

          <div style={{ position: "relative" }}>
            <FormField
              label="Passwort"
              name="password"
              type={showPassword ? "text" : "password"}
              value={password}
              onChange={(event: ChangeEvent<HTMLInputElement>) => setPassword(event.target.value)}
              onBlur={() => setTouched((state) => ({ ...state, password: true }))}
              error={errors.password}
              touched={showFieldError("password")}
              autoComplete="current-password"
            />
            <button
              type="button"
              aria-label={showPassword ? "Passwort verbergen" : "Passwort anzeigen"}
              aria-pressed={showPassword}
              onClick={() => setShowPassword((value) => !value)}
              style={{
                position: "absolute",
                right: "var(--space-0)",
                top: "24px",
                width: "44px",
                height: "44px",
                display: "inline-flex",
                alignItems: "center",
                justifyContent: "center",
                background: "transparent",
                border: "none",
                borderRadius: "var(--radius-sm)",
                color: "var(--color-fgMuted)",
                cursor: "pointer",
              }}
            >
              {showPassword ? (
                <svg
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  <path d="M17.94 17.94A10.07 10.07 0 0 1 12 20c-7 0-11-8-11-8a18.45 18.45 0 0 1 5.06-5.94" />
                  <path d="M9.9 4.24A9.12 9.12 0 0 1 12 4c7 0 11 8 11 8a18.5 18.5 0 0 1-2.16 3.19" />
                  <path d="M14.12 14.12a3 3 0 1 1-4.24-4.24" />
                  <line x1="1" y1="1" x2="23" y2="23" />
                </svg>
              ) : (
                <svg
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  aria-hidden="true"
                >
                  <path d="M1 12s4-7 11-7 11 7 11 7-4 7-11 7S1 12 1 12Z" />
                  <circle cx="12" cy="12" r="3" />
                </svg>
              )}
            </button>
          </div>

          <button
            type="submit"
            className="btn btn--primary"
            style={{ width: "100%" }}
            disabled={submitting}
            aria-busy={submitting || undefined}
          >
            {submitting ? "Anmeldung läuft …" : "Anmelden"}
          </button>
        </form>
      </div>
    </section>
  );
}
