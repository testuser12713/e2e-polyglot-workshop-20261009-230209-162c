export type OrderStatus =
  | "requested"
  | "confirmed"
  | "in_progress"
  | "done"
  | "picked_up";

const STATUS_LABELS: Record<OrderStatus, string> = {
  requested: "angefragt",
  confirmed: "bestätigt",
  in_progress: "in Arbeit",
  done: "fertig",
  picked_up: "abgeholt",
};

export const ORDER_STATUSES: readonly OrderStatus[] = [
  "requested",
  "confirmed",
  "in_progress",
  "done",
  "picked_up",
];

export function statusLabel(status: string): string {
  if (status in STATUS_LABELS) {
    return STATUS_LABELS[status as OrderStatus];
  }
  return status;
}

export function formatEuro(cents: number): string {
  const value = Math.round(cents);
  const negative = value < 0;
  const abs = Math.abs(value);
  const euros = Math.floor(abs / 100);
  const rest = abs % 100;
  const eurosStr = euros.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ".");
  const restStr = rest.toString().padStart(2, "0");
  return `${negative ? "-" : ""}${eurosStr},${restStr}\u00A0€`;
}

function pad(value: number): string {
  return value.toString().padStart(2, "0");
}

export function formatDateTime(iso: string): string {
  if (!iso) {
    return "";
  }
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const day = pad(date.getDate());
  const month = pad(date.getMonth() + 1);
  const year = date.getFullYear();
  const hours = pad(date.getHours());
  const minutes = pad(date.getMinutes());
  return `${day}.${month}.${year}, ${hours}:${minutes} Uhr`;
}
