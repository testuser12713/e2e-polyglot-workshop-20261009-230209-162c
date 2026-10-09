import { useEffect, useMemo, useRef, useState, type ChangeEvent, type FormEvent } from "react";
import { Link } from "react-router-dom";
import ErrorBanner from "../components/ErrorBanner";
import FormField from "../components/FormField";
import { ApiError, apiFetch } from "../lib/api";
import {
  EMPTY_APPOINTMENT,
  toAppointmentRequest,
  validateAppointment,
  type AppointmentField,
  type AppointmentFormValues,
  type AppointmentResponse,
} from "../lib/appointment";

export default function AppointmentPage() {
  const [values, setValues] = useState<AppointmentFormValues>(EMPTY_APPOINTMENT);
  const [touched, setTouched] = useState<Partial<Record<AppointmentField, boolean>>>({});
  const [submitAttempted, setSubmitAttempted] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [serverError, setServerError] = useState<string | null>(null);
  const [result, setResult] = useState<AppointmentResponse | null>(null);
  const formRef = useRef<HTMLFormElement>(null);

  const errors = useMemo(() => validateAppointment(values), [values]);
  const hasErrors = Object.keys(errors).length > 0;

  useEffect(() => {
    if (!submitAttempted || !hasErrors) {
      return;
    }
    const firstInvalid = formRef.current?.querySelector<HTMLElement>('[aria-invalid="true"]');
    firstInvalid?.focus();
  }, [submitAttempted, hasErrors]);

  function handleChange(event: ChangeEvent<HTMLInputElement>) {
    const { name, value } = event.target;
    setValues((previous) => ({
      ...previous,
      [name]: name === "plate" ? value.toUpperCase() : value,
    }));
  }

  function handleBlur(field: AppointmentField) {
    setTouched((previous) => ({ ...previous, [field]: true }));
  }

  function fieldError(field: AppointmentField): string | null {
    if (!touched[field] && !submitAttempted) {
      return null;
    }
    return errors[field] ?? null;
  }

  async function handleSubmit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSubmitAttempted(true);
    setServerError(null);
    if (Object.keys(validateAppointment(values)).length > 0) {
      return;
    }

    setSubmitting(true);
    try {
      const response = await apiFetch<AppointmentResponse>("/api/orders", {
        method: "POST",
        body: toAppointmentRequest(values),
      });
      setResult(response);
    } catch (error) {
      setServerError(
        error instanceof ApiError
          ? error.message
          : "Die Anfrage konnte nicht übermittelt werden. Bitte versuchen Sie es später erneut.",
      );
    } finally {
      setSubmitting(false);
    }
  }

  if (result) {
    return (
      <section className="page page--narrow">
        <header className="page__header">
          <h1 className="page__title">Termin anfragen</h1>
        </header>
        <div className="card" role="status">
          <h2 className="card__title">Vielen Dank für Ihre Anfrage!</h2>
          <p className="muted">Ihre Auftragsnummer:</p>
          <p
            style={{
              fontFamily: "var(--font-mono)",
              fontSize: "var(--size-2xl)",
              fontWeight: 600,
              color: "var(--color-accent)",
              letterSpacing: "0.02em",
              overflowWrap: "anywhere",
              margin: "var(--space-1) 0 var(--space-3)",
            }}
          >
            {result.order_number}
          </p>
          <p className="muted">
            Wir haben Ihre Anfrage erhalten und melden uns in Kürze mit einer Bestätigung.
          </p>
          <p style={{ marginTop: "var(--space-3)" }}>
            <Link className="btn btn--secondary" to="/mein-auftrag">
              Status abrufen
            </Link>
          </p>
        </div>
      </section>
    );
  }

  return (
    <section className="page page--narrow">
      <header className="page__header">
        <h1 className="page__title">Termin anfragen</h1>
        <p className="page__subline">
          Geben Sie Ihre Fahrzeug- und Kontaktdaten an, um einen Werkstatttermin anzufragen.
        </p>
      </header>

      {submitAttempted && hasErrors ? (
        <ErrorBanner error="Bitte prüfen Sie die markierten Felder." />
      ) : null}

      {serverError ? <ErrorBanner error={serverError} /> : null}

      <div className="card">
        <form ref={formRef} className="appointment-form" onSubmit={handleSubmit} noValidate>
          <FormField
            label="Name"
            name="name"
            value={values.name}
            onChange={handleChange}
            onBlur={() => handleBlur("name")}
            error={fieldError("name")}
            touched={touched.name || submitAttempted}
            required
            autoComplete="name"
          />
          <FormField
            label="E-Mail"
            name="email"
            type="email"
            value={values.email}
            onChange={handleChange}
            onBlur={() => handleBlur("email")}
            error={fieldError("email")}
            touched={touched.email || submitAttempted}
            required
            autoComplete="email"
            inputMode="email"
          />
          <FormField
            label="Telefon"
            name="phone"
            type="tel"
            value={values.phone}
            onChange={handleChange}
            onBlur={() => handleBlur("phone")}
            error={fieldError("phone")}
            touched={touched.phone || submitAttempted}
            required
            autoComplete="tel"
            inputMode="tel"
          />
          <FormField
            label="Kennzeichen"
            name="plate"
            value={values.plate}
            onChange={handleChange}
            onBlur={() => handleBlur("plate")}
            error={fieldError("plate")}
            touched={touched.plate || submitAttempted}
            required
            placeholder="z. B. B-AB 1234"
            autoComplete="off"
            maxLength={12}
          />
          <FormField
            label="Marke"
            name="brand"
            value={values.brand}
            onChange={handleChange}
            onBlur={() => handleBlur("brand")}
            error={fieldError("brand")}
            touched={touched.brand || submitAttempted}
            required
            autoComplete="off"
          />
          <FormField
            label="Modell"
            name="model"
            value={values.model}
            onChange={handleChange}
            onBlur={() => handleBlur("model")}
            error={fieldError("model")}
            touched={touched.model || submitAttempted}
            required
            autoComplete="off"
          />
          <FormField
            label="Kilometerstand"
            name="mileage"
            value={values.mileage}
            onChange={handleChange}
            onBlur={() => handleBlur("mileage")}
            error={fieldError("mileage")}
            touched={touched.mileage || submitAttempted}
            required
            hint="in km"
            inputMode="numeric"
            autoComplete="off"
          />
          <FormField
            label="Wunschtermin"
            name="desiredDate"
            type="date"
            value={values.desiredDate}
            onChange={handleChange}
            onBlur={() => handleBlur("desiredDate")}
            error={fieldError("desiredDate")}
            touched={touched.desiredDate || submitAttempted}
            required
          />
          <FormField
            label="Problembeschreibung"
            name="description"
            value={values.description}
            onChange={handleChange}
            onBlur={() => handleBlur("description")}
            error={fieldError("description")}
            touched={touched.description || submitAttempted}
            required
          />
          <button type="submit" className="btn btn--primary" disabled={submitting} aria-busy={submitting}>
            {submitting ? "Wird gesendet …" : "Termin anfragen"}
          </button>
        </form>
      </div>
    </section>
  );
}
