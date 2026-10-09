import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ChangeEvent,
  type FormEvent,
  type KeyboardEvent,
} from "react";
import { Link, useParams } from "react-router-dom";
import ErrorBanner from "../components/ErrorBanner";
import { ApiError, apiFetch } from "../lib/api";
import { formatDateTime, formatEuro, statusLabel } from "../lib/format";
import "./WorkshopOrderDetailPage.css";

interface Customer {
  id: number;
  name: string;
  email: string;
  phone: string;
}

interface Vehicle {
  id: number;
  customer_id: number;
  plate: string;
  brand: string;
  model: string;
  mileage_km: number;
}

interface OrderItem {
  id: number;
  kind: "labor" | "part";
  description: string;
  hours: number | null;
  quantity: number | null;
  unit_price_cents: number | null;
}

interface HistoryEntry {
  status: string;
  changed_at: string;
}

interface OrderDetail {
  order_number: string;
  status: string;
  desired_date: string;
  description: string;
  customer: Customer | null;
  vehicle: Vehicle | null;
  items: OrderItem[];
  history: HistoryEntry[];
  invoice: unknown | null;
}

interface StatusResponse {
  order_number: string;
  status: string;
  history: HistoryEntry[];
}

type Banner = { code?: string; message: string } | null;

type PositionKind = "labor" | "part";

interface LaborValues {
  hours: string;
}

interface PartValues {
  description: string;
  quantity: string;
  unitPrice: string;
}

const BADGE_CLASS: Record<string, string> = {
  requested: "status-badge--requested",
  confirmed: "status-badge--confirmed",
  in_progress: "status-badge--in_progress",
  done: "status-badge--done",
  picked_up: "status-badge--picked_up",
};

const NEXT_STATUS: Record<string, string | undefined> = {
  requested: "confirmed",
  confirmed: "in_progress",
  in_progress: "done",
  done: "picked_up",
};

const ACTION_LABEL: Record<string, string> = {
  confirmed: "Auftrag bestätigen",
  in_progress: "In Arbeit setzen",
  done: "Als fertig melden",
  picked_up: "Als abgeholt markieren",
};

const SUCCESS_MESSAGE: Record<string, string> = {
  confirmed: "Auftrag bestätigt.",
  in_progress: "Auftrag in Arbeit gesetzt.",
  done: "Auftrag als fertig gemeldet.",
  picked_up: "Auftrag als abgeholt markiert.",
};

const LOAD_FALLBACK = "Der Auftrag konnte nicht geladen werden.";
const ACTION_FALLBACK = "Der Statuswechsel konnte nicht durchgeführt werden.";
const ITEM_FALLBACK = "Die Position konnte nicht gespeichert werden.";

function pad(value: number): string {
  return value.toString().padStart(2, "0");
}

function formatDate(value: string): string {
  if (!value) {
    return "–";
  }
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  if (match) {
    return `${match[3]}.${match[2]}.${match[1]}`;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return `${pad(date.getDate())}.${pad(date.getMonth() + 1)}.${date.getFullYear()}`;
}

function formatHours(hours: number): string {
  if (!Number.isFinite(hours)) {
    return "–";
  }
  return `${hours.toFixed(1).replace(".", ",")} h`;
}

function capitalize(value: string): string {
  if (!value) {
    return value;
  }
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function parseDecimal(value: string): number | null {
  const normalized = value.trim().replace(",", ".");
  if (!/^\d+(\.\d{1,2})?$/.test(normalized)) {
    return null;
  }
  const parsed = Number(normalized);
  return Number.isFinite(parsed) ? parsed : null;
}

function toCents(value: string): number | null {
  const parsed = parseDecimal(value);
  if (parsed === null) {
    return null;
  }
  return Math.round(parsed * 100);
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`status-badge ${BADGE_CLASS[status] ?? "status-badge--requested"}`}>
      <span className="status-badge__dot" aria-hidden="true" />
      {statusLabel(status)}
    </span>
  );
}

function StatusTimeline({ history }: { history: HistoryEntry[] }) {
  const entries = useMemo(
    () =>
      [...history].sort((a, b) => {
        const left = new Date(a.changed_at).getTime();
        const right = new Date(b.changed_at).getTime();
        return right - left;
      }),
    [history],
  );

  if (entries.length === 0) {
    return <p className="muted">Noch keine Statuswechsel.</p>;
  }

  return (
    <ul className="timeline">
      {entries.map((entry, index) => (
        <li className="timeline__item" key={`${entry.status}-${entry.changed_at}-${index}`}>
          <StatusBadge status={entry.status} />
          <time className="timeline__time" dateTime={entry.changed_at}>
            {formatDateTime(entry.changed_at)}
          </time>
        </li>
      ))}
    </ul>
  );
}

function PositionList({ items }: { items: OrderItem[] }) {
  if (items.length === 0) {
    return <p className="muted">Noch keine Positionen erfasst.</p>;
  }

  return (
    <table className="positions-table">
      <thead>
        <tr>
          <th scope="col">Bezeichnung</th>
          <th scope="col" className="num">
            Menge
          </th>
          <th scope="col" className="num">
            Einzelpreis
          </th>
          <th scope="col" className="num">
            Summe
          </th>
        </tr>
      </thead>
      <tbody>
        {items.map((item) => {
          const isLabor = item.kind === "labor";
          const quantityLabel = isLabor
            ? item.hours !== null
              ? formatHours(item.hours)
              : "–"
            : item.quantity !== null
              ? item.quantity.toString()
              : "–";
          const unitLabel =
            !isLabor && item.unit_price_cents !== null
              ? formatEuro(item.unit_price_cents)
              : "–";
          const sumLabel =
            !isLabor && item.quantity !== null && item.unit_price_cents !== null
              ? formatEuro(Math.round(item.quantity * item.unit_price_cents))
              : "–";
          return (
            <tr key={item.id}>
              <td data-label="Bezeichnung">{item.description}</td>
              <td data-label="Menge" className="num">
                {quantityLabel}
              </td>
              <td data-label="Einzelpreis" className="num">
                {unitLabel}
              </td>
              <td data-label="Summe" className="num">
                {sumLabel}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

export default function WorkshopOrderDetailPage() {
  const { orderNumber = "" } = useParams();

  const [order, setOrder] = useState<OrderDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<Banner>(null);
  const [actionError, setActionError] = useState<Banner>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [pendingStatus, setPendingStatus] = useState<string | null>(null);
  const [transitioning, setTransitioning] = useState(false);

  const [kind, setKind] = useState<PositionKind>("labor");
  const [labor, setLabor] = useState<LaborValues>({ hours: "" });
  const [part, setPart] = useState<PartValues>({ description: "", quantity: "", unitPrice: "" });
  const [touched, setTouched] = useState<Record<string, boolean>>({});
  const [submitted, setSubmitted] = useState(false);
  const [addingItem, setAddingItem] = useState(false);

  const modalRef = useRef<HTMLDivElement>(null);

  const loadOrder = useCallback(
    async (signal?: AbortSignal) => {
      setLoading(true);
      setLoadError(null);
      try {
        const data = await apiFetch<OrderDetail>(`/api/orders/${encodeURIComponent(orderNumber)}`, {
          signal,
        });
        setOrder({
          ...data,
          items: Array.isArray(data.items) ? data.items : [],
          history: Array.isArray(data.history) ? data.history : [],
        });
      } catch (error) {
        if ((error as Error)?.name === "AbortError") {
          return;
        }
        if (error instanceof ApiError) {
          setLoadError({ code: error.code, message: error.message });
        } else {
          setLoadError({ message: LOAD_FALLBACK });
        }
      } finally {
        setLoading(false);
      }
    },
    [orderNumber],
  );

  useEffect(() => {
    const controller = new AbortController();
    void loadOrder(controller.signal);
    return () => controller.abort();
  }, [loadOrder]);

  const nextStatus = order ? NEXT_STATUS[order.status] : undefined;

  const markTouched = (field: string) => {
    setTouched((previous) => ({ ...previous, [field]: true }));
  };

  const showFieldError = (field: string, error: string | null): string | null => {
    if (!touched[field] && !submitted) {
      return null;
    }
    return error;
  };

  const laborHoursError = (() => {
    const value = parseDecimal(labor.hours);
    if (value === null || value <= 0) {
      return "Bitte geben Sie eine gültige Stundenzahl größer 0 ein.";
    }
    return null;
  })();

  const partDescriptionError = part.description.trim()
    ? null
    : "Bitte geben Sie eine Bezeichnung ein.";
  const partQuantityError = /^\d+$/.test(part.quantity.trim()) && Number(part.quantity) >= 1
    ? null
    : "Bitte geben Sie eine Menge von mindestens 1 ein.";
  const partPriceError = toCents(part.unitPrice) === null
    ? "Bitte geben Sie einen gültigen Einzelpreis ein."
    : null;

  const openConfirm = (target: string) => {
    setActionError(null);
    setPendingStatus(target);
  };

  useEffect(() => {
    if (!pendingStatus) {
      return;
    }
    const previous = document.activeElement as HTMLElement | null;
    const confirmButton = modalRef.current?.querySelector<HTMLButtonElement>("[data-autofocus]");
    confirmButton?.focus();

    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key === "Escape") {
        event.preventDefault();
        setPendingStatus(null);
      }
    };
    document.addEventListener("keydown", onKeyDown);
    return () => {
      document.removeEventListener("keydown", onKeyDown);
      previous?.focus();
    };
  }, [pendingStatus]);

  const handleModalKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "Tab") {
      return;
    }
    const focusable = modalRef.current?.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
    );
    if (!focusable || focusable.length === 0) {
      return;
    }
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  };

  const confirmStatus = async () => {
    if (!pendingStatus) {
      return;
    }
    const target = pendingStatus;
    setTransitioning(true);
    setActionError(null);
    setSuccess(null);
    try {
      const response = await apiFetch<StatusResponse>(
        `/api/orders/${encodeURIComponent(orderNumber)}/status`,
        { method: "POST", body: { status: target } },
      );
      setOrder((previous) =>
        previous
          ? {
              ...previous,
              status: response.status,
              history: Array.isArray(response.history) ? response.history : previous.history,
            }
          : previous,
      );
      setSuccess(SUCCESS_MESSAGE[target] ?? "Status aktualisiert.");
      setPendingStatus(null);
    } catch (error) {
      setPendingStatus(null);
      if (error instanceof ApiError) {
        setActionError({ code: error.code, message: error.message });
      } else {
        setActionError({ message: ACTION_FALLBACK });
      }
    } finally {
      setTransitioning(false);
    }
  };

  const addPosition = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setSubmitted(true);
    setSuccess(null);
    setActionError(null);

    if (kind === "labor" && laborHoursError) {
      return;
    }
    if (kind === "part" && (partDescriptionError || partQuantityError || partPriceError)) {
      return;
    }

    setAddingItem(true);
    try {
      let body: Record<string, unknown>;
      if (kind === "labor") {
        body = {
          kind: "labor",
          description: "Arbeitszeit",
          hours: parseDecimal(labor.hours),
        };
      } else {
        body = {
          kind: "part",
          description: part.description.trim(),
          quantity: Number(part.quantity),
          unit_price_cents: toCents(part.unitPrice),
        };
      }

      const created = await apiFetch<OrderItem>(
        `/api/orders/${encodeURIComponent(orderNumber)}/items`,
        { method: "POST", body },
      );
      setOrder((previous) =>
        previous ? { ...previous, items: [...(previous.items ?? []), created] } : previous,
      );
      setLabor({ hours: "" });
      setPart({ description: "", quantity: "", unitPrice: "" });
      setTouched({});
      setSubmitted(false);
      setSuccess("Position hinzugefügt.");
    } catch (error) {
      if (error instanceof ApiError) {
        setActionError({ code: error.code, message: error.message });
      } else {
        setActionError({ message: ITEM_FALLBACK });
      }
    } finally {
      setAddingItem(false);
    }
  };

  return (
    <section className="page">
      <header className="page__header">
        <h1 className="page__title">
          Auftrag {orderNumber ? <span className="detail__order-number">{orderNumber}</span> : null}
        </h1>
        <p className="page__subline">
          Auftrag bestätigen, Positionen erfassen und den Status weitersetzen.
        </p>
      </header>

      {nextStatus ? (
        <div className="detail-action-bar">
          <button
            type="button"
            className="btn btn--primary detail-action-bar__button"
            onClick={() => openConfirm(nextStatus)}
          >
            {ACTION_LABEL[nextStatus] ?? `Auf „${capitalize(statusLabel(nextStatus))}" setzen`}
          </button>
        </div>
      ) : null}

      {loadError ? <ErrorBanner error={loadError} /> : null}

      {success ? (
        <ErrorBanner error={success} variant="success" onClose={() => setSuccess(null)} />
      ) : null}

      {actionError ? (
        <ErrorBanner
          error={actionError}
          variant="error"
          onClose={() => setActionError(null)}
        />
      ) : null}

      {loading ? (
        <div className="card detail-skeleton" aria-busy="true">
          <span className="visually-hidden">Wird geladen …</span>
          <span className="skeleton" />
          <span className="skeleton" />
          <span className="skeleton" />
        </div>
      ) : null}

      {!loading && order ? (
        <>
          <section className="card">
            <div className="detail__head">
              <div className="detail__numbers">
                <span className="detail__order-number">{order.order_number}</span>
                {order.vehicle ? (
                  <span className="detail__plate">{order.vehicle.plate}</span>
                ) : null}
              </div>
              <StatusBadge status={order.status} />
            </div>

            <dl className="detail__meta">
              <div>
                <dt>Wunschtermin</dt>
                <dd>{formatDate(order.desired_date)}</dd>
              </div>
              <div>
                <dt>Problembeschreibung</dt>
                <dd>{order.description || "–"}</dd>
              </div>
            </dl>
          </section>

          <section className="card">
            <h2 className="card__title">Kunde</h2>
            {order.customer ? (
              <dl className="detail__meta">
                <div>
                  <dt>Name</dt>
                  <dd>{order.customer.name}</dd>
                </div>
                <div>
                  <dt>E-Mail</dt>
                  <dd>{order.customer.email}</dd>
                </div>
                <div>
                  <dt>Telefon</dt>
                  <dd>{order.customer.phone}</dd>
                </div>
              </dl>
            ) : (
              <p className="muted">Keine Kundendaten hinterlegt.</p>
            )}
          </section>

          <section className="card">
            <h2 className="card__title">Fahrzeug</h2>
            {order.vehicle ? (
              <dl className="detail__meta">
                <div>
                  <dt>Kennzeichen</dt>
                  <dd className="mono">{order.vehicle.plate}</dd>
                </div>
                <div>
                  <dt>Fahrzeug</dt>
                  <dd>
                    {order.vehicle.brand} {order.vehicle.model}
                  </dd>
                </div>
                <div>
                  <dt>Kilometerstand</dt>
                  <dd className="num-units">{order.vehicle.mileage_km} km</dd>
                </div>
              </dl>
            ) : (
              <p className="muted">Keine Fahrzeugdaten hinterlegt.</p>
            )}
          </section>

          <section className="card">
            <h2 className="card__title">Positionen</h2>
            <PositionList items={order.items} />
          </section>

          <section className="card">
            <h2 className="card__title">Verlauf</h2>
            <StatusTimeline history={order.history} />
          </section>

          <section className="card">
            <h2 className="card__title">Position hinzufügen</h2>
            <form className="position-form" onSubmit={addPosition} noValidate>
              <div className="segmented" role="group" aria-label="Positionsart">
                <button
                  type="button"
                  aria-pressed={kind === "labor"}
                  className={`segmented__tab${kind === "labor" ? " segmented__tab--active" : ""}`}
                  onClick={() => setKind("labor")}
                >
                  Arbeitszeit
                </button>
                <button
                  type="button"
                  aria-pressed={kind === "part"}
                  className={`segmented__tab${kind === "part" ? " segmented__tab--active" : ""}`}
                  onClick={() => setKind("part")}
                >
                  Teil
                </button>
              </div>

              {kind === "labor" ? (
                <div className={`field${showFieldError("hours", laborHoursError) ? " field--error" : ""}`}>
                  <label className="field__label" htmlFor="position-hours">
                    Stunden
                  </label>
                  <input
                    id="position-hours"
                    name="hours"
                    type="text"
                    inputMode="decimal"
                    className="field__input"
                    value={labor.hours}
                    placeholder="z. B. 2,5"
                    onChange={(event: ChangeEvent<HTMLInputElement>) =>
                      setLabor({ hours: event.target.value })
                    }
                    onBlur={() => markTouched("hours")}
                    aria-invalid={showFieldError("hours", laborHoursError) ? true : undefined}
                  />
                  {showFieldError("hours", laborHoursError) ? (
                    <p className="field__error" role="alert">
                      {laborHoursError}
                    </p>
                  ) : null}
                </div>
              ) : (
                <>
                  <div
                    className={`field${showFieldError("description", partDescriptionError) ? " field--error" : ""}`}
                  >
                    <label className="field__label" htmlFor="position-description">
                      Bezeichnung
                    </label>
                    <input
                      id="position-description"
                      name="description"
                      type="text"
                      className="field__input"
                      value={part.description}
                      onChange={(event: ChangeEvent<HTMLInputElement>) =>
                        setPart((previous) => ({ ...previous, description: event.target.value }))
                      }
                      onBlur={() => markTouched("description")}
                      aria-invalid={
                        showFieldError("description", partDescriptionError) ? true : undefined
                      }
                    />
                    {showFieldError("description", partDescriptionError) ? (
                      <p className="field__error" role="alert">
                        {partDescriptionError}
                      </p>
                    ) : null}
                  </div>

                  <div
                    className={`field${showFieldError("quantity", partQuantityError) ? " field--error" : ""}`}
                  >
                    <label className="field__label" htmlFor="position-quantity">
                      Menge
                    </label>
                    <input
                      id="position-quantity"
                      name="quantity"
                      type="text"
                      inputMode="numeric"
                      className="field__input"
                      value={part.quantity}
                      onChange={(event: ChangeEvent<HTMLInputElement>) =>
                        setPart((previous) => ({ ...previous, quantity: event.target.value }))
                      }
                      onBlur={() => markTouched("quantity")}
                      aria-invalid={showFieldError("quantity", partQuantityError) ? true : undefined}
                    />
                    {showFieldError("quantity", partQuantityError) ? (
                      <p className="field__error" role="alert">
                        {partQuantityError}
                      </p>
                    ) : null}
                  </div>

                  <div
                    className={`field${showFieldError("unitPrice", partPriceError) ? " field--error" : ""}`}
                  >
                    <label className="field__label" htmlFor="position-price">
                      Einzelpreis
                    </label>
                    <div className="price-input">
                      <input
                        id="position-price"
                        name="unitPrice"
                        type="text"
                        inputMode="decimal"
                        className="field__input price-input__control"
                        value={part.unitPrice}
                        placeholder="z. B. 89,00"
                        onChange={(event: ChangeEvent<HTMLInputElement>) =>
                          setPart((previous) => ({ ...previous, unitPrice: event.target.value }))
                        }
                        onBlur={() => markTouched("unitPrice")}
                        aria-invalid={showFieldError("unitPrice", partPriceError) ? true : undefined}
                      />
                      <span className="price-input__suffix" aria-hidden="true">
                        €
                      </span>
                    </div>
                    {showFieldError("unitPrice", partPriceError) ? (
                      <p className="field__error" role="alert">
                        {partPriceError}
                      </p>
                    ) : null}
                  </div>
                </>
              )}

              <button
                type="submit"
                className="btn btn--primary position-form__submit"
                disabled={addingItem}
                aria-busy={addingItem}
              >
                {addingItem ? "Wird gespeichert …" : "Position hinzufügen"}
              </button>
            </form>
          </section>
        </>
      ) : null}

      {!loading && !order && !loadError ? (
        <div className="card empty-state">
          <p className="empty-state__title">Der Auftrag wurde nicht gefunden.</p>
          <p className="empty-state__text">
            <Link to="/werkstatt/auftraege">Zurück zur Auftragsliste</Link>
          </p>
        </div>
      ) : null}

      {pendingStatus ? (
        <div
          className="modal-overlay"
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) {
              setPendingStatus(null);
            }
          }}
        >
          <div
            className="modal"
            role="dialog"
            aria-modal="true"
            aria-labelledby="confirm-status-title"
            ref={modalRef}
            onKeyDown={handleModalKeyDown}
          >
            <h2 className="modal__title" id="confirm-status-title">
              Auftrag auf „{capitalize(statusLabel(pendingStatus))}" setzen?
            </h2>
            {pendingStatus === "done" ? (
              <p className="modal__text">Danach wird automatisch eine Rechnung erstellt.</p>
            ) : null}
            <div className="modal__actions">
              <button
                type="button"
                className="btn btn--secondary"
                onClick={() => setPendingStatus(null)}
                disabled={transitioning}
              >
                Abbrechen
              </button>
              <button
                type="button"
                className="btn btn--primary"
                data-autofocus
                onClick={() => void confirmStatus()}
                disabled={transitioning}
                aria-busy={transitioning}
              >
                {transitioning ? "Wird gespeichert …" : "Bestätigen"}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </section>
  );
}
