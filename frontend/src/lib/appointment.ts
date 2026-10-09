export interface AppointmentFormValues {
  name: string;
  email: string;
  phone: string;
  plate: string;
  brand: string;
  model: string;
  mileage: string;
  desiredDate: string;
  description: string;
}

export type AppointmentField = keyof AppointmentFormValues;

export type AppointmentErrors = Partial<Record<AppointmentField, string>>;

export interface AppointmentRequest {
  name: string;
  email: string;
  phone: string;
  plate: string;
  brand: string;
  model: string;
  mileage_km: number;
  desired_date: string;
  description: string;
}

export interface AppointmentResponse {
  order_number: string;
  status: string;
}

export const EMPTY_APPOINTMENT: AppointmentFormValues = {
  name: "",
  email: "",
  phone: "",
  plate: "",
  brand: "",
  model: "",
  mileage: "",
  desiredDate: "",
  description: "",
};

export const APPOINTMENT_MESSAGES: Record<AppointmentField, string> = {
  name: "Bitte geben Sie Ihren Namen ein.",
  email: "Bitte geben Sie eine gültige E-Mail-Adresse ein.",
  phone: "Bitte geben Sie eine gültige Telefonnummer ein.",
  plate: "Bitte geben Sie ein gültiges Kennzeichen ein.",
  brand: "Bitte geben Sie die Marke ein.",
  model: "Bitte geben Sie das Modell ein.",
  mileage: "Bitte geben Sie einen gültigen Kilometerstand ein.",
  desiredDate: "Bitte wählen Sie einen Wunschtermin in der Zukunft.",
  description: "Bitte geben Sie eine Problembeschreibung ein.",
};

const EMAIL_PATTERN = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
const PHONE_PATTERN = /^[+0-9][0-9 ()/.-]{5,}$/;
const PLATE_PATTERN = /^[A-ZÄÖÜ]{1,3}[- ]?[A-Z]{1,2}[- ]?[0-9]{1,4}[EH]?$/;

export function normalizePlate(value: string): string {
  return value.trim().toUpperCase();
}

export function parseMileage(value: string): number | null {
  const digits = value.replace(/[^0-9]/g, "");
  if (!digits) {
    return null;
  }
  const parsed = Number.parseInt(digits, 10);
  if (!Number.isFinite(parsed) || parsed < 0) {
    return null;
  }
  return parsed;
}

function startOfDay(now: Date): number {
  return new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
}

export function isFutureDate(value: string, now: Date = new Date()): boolean {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    return false;
  }
  const [year, month, day] = value.split("-").map((part) => Number.parseInt(part, 10));
  const date = new Date(year, month - 1, day);
  if (
    date.getFullYear() !== year ||
    date.getMonth() !== month - 1 ||
    date.getDate() !== day
  ) {
    return false;
  }
  return date.getTime() > startOfDay(now);
}

export function validateAppointment(
  values: AppointmentFormValues,
  now: Date = new Date(),
): AppointmentErrors {
  const errors: AppointmentErrors = {};

  if (!values.name.trim()) {
    errors.name = APPOINTMENT_MESSAGES.name;
  }
  if (!EMAIL_PATTERN.test(values.email.trim())) {
    errors.email = APPOINTMENT_MESSAGES.email;
  }
  if (!PHONE_PATTERN.test(values.phone.trim())) {
    errors.phone = APPOINTMENT_MESSAGES.phone;
  }
  if (!PLATE_PATTERN.test(normalizePlate(values.plate))) {
    errors.plate = APPOINTMENT_MESSAGES.plate;
  }
  if (!values.brand.trim()) {
    errors.brand = APPOINTMENT_MESSAGES.brand;
  }
  if (!values.model.trim()) {
    errors.model = APPOINTMENT_MESSAGES.model;
  }
  if (parseMileage(values.mileage) === null) {
    errors.mileage = APPOINTMENT_MESSAGES.mileage;
  }
  if (!isFutureDate(values.desiredDate, now)) {
    errors.desiredDate = APPOINTMENT_MESSAGES.desiredDate;
  }
  if (!values.description.trim()) {
    errors.description = APPOINTMENT_MESSAGES.description;
  }

  return errors;
}

export function toAppointmentRequest(values: AppointmentFormValues): AppointmentRequest {
  return {
    name: values.name.trim(),
    email: values.email.trim(),
    phone: values.phone.trim(),
    plate: normalizePlate(values.plate),
    brand: values.brand.trim(),
    model: values.model.trim(),
    mileage_km: parseMileage(values.mileage) ?? 0,
    desired_date: values.desiredDate,
    description: values.description.trim(),
  };
}
