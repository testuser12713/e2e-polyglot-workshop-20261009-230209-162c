import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type ChangeEvent,
  type FormEvent,
  type RefObject,
} from "react";
import ErrorBanner from "../components/ErrorBanner";
import FormField from "../components/FormField";
import { ApiError, apiFetch } from "../lib/api";
import { formatDateTime, formatEuro, statusLabel } from "../lib/format";
import "./OrderLookupPage.css";

interface HistoryEntry {
  status: string;
  changed_at: string;
}

interface PublicOrder {
  order_number: string;
  status: string;
  desired_date: string;
  description: string;
  plate: string;
  history: HistoryEntry[];
}

interface InvoiceItem {
  description: string;
  quantity: number;
  unit_price_cents: number;
}

interface Invoice {
  invoice_number: string;
  issued_at: string;
  net_cents: number;
  vat_cents: number;
  gross_cents: number;
  items: InvoiceItem[];
}

interface InvoiceResponse {
  invoice: Invoice | null;
}

type Banner = { variant: "error" | "warning"; message: string; code?: string } | null;

type InvoiceState =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "ready"; invoice: Invoice | null }
  | { kind: "error"; message: string; code?: string };

const NOT_FOUND_MESSAGE =
  "Zu dieser Kombination aus Auftragsnummer und Kennzeichen wurde kein Auftrag gefunden.";
const RATE_LIMIT_MESSAGE =
  "Zu viele Versuche. Bitte versuchen Sie es in einer Minute erneut.";

const BADGE_CLASS: Record<string, string> = {
  requested: "status-badge--requested",
  confirmed: "status-badge--confirmed",
  in_progress: "status-badge--in_progress",
  done: "status-badge--done",
  picked_up: "status-badge--picked_up",
};

function formatDate(value: string): string {
  if (!value) {
    return "";
  }
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  if (match) {
    return `${match[3]}.${match[2]}.${match[1]}`;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  const day = date.getDate().toString().padStart(2, "0");
  const month = (date.getMonth() + 1).toString().padStart(2, "0");
  return `${day}.${month}.${date.getFullYear()}`;
}

function formatQuantity(quantity: number): string {
  if (!Number.isFinite(quantity)) {
    return "";
  }
  if (Number.isInteger(quantity)) {
    return quantity.toString();
  }
  return `${quantity.toFixed(1).replace(".", ",")} h`;
}

function normalizePlate(value: string): string {
  return value.toUpperCase().replace(/[\s.-]/g, "");
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
  const entries = [...history].sort((a, b) => {
    const left = new Date(a.changed_at).getTime();
    const right = new Date(b.changed_at).getTime();
    return right - left;
  });

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

function InvoiceView({
  state,
  sectionRef,
}: {
  state: InvoiceState;
  sectionRef: RefObject<HTMLElement | null>;
}) {
  if (state.kind === "idle") {
    return null;
  }

  return (
    <section className="card" id="rechnung" ref={sectionRef}>
      <h2 className="card__title">Rechnung</h2>

      {state.kind === "loading" ? (
        <div className="invoice__skeleton" aria-busy="true">
          <span className="skeleton" />
          <span className="skeleton" />
          <span className="skeleton" />
          <span className="muted">Wird geladen …</span>
        </div>
      ) : null}

      {state.kind === "error" ? (
        <div className="lookup__invoice-alert">
          <ErrorBanner error={{ code: state.code, message: state.message }} />
        </div>
      ) : null}

      {state.kind === "ready" && state.invoice === null ? (
        <div className="empty-state">
          <p className="empty-state__title">Die Rechnung liegt noch nicht vor.</p>
          <p className="empty-state__text">
            Sie erscheint hier, sobald der Auftrag fertiggestellt ist.
          </p>
        </div>
      ) : null}

      {state.kind === "ready" && state.invoice !== null ? (
        <>
          <p className="invoice__number mono">Rechnung Nr. {state.invoice.invoice_number}</p>
          {state.invoice.issued_at ? (
            <p className="invoice__issued">
              Ausgestellt am {formatDate(state.invoice.issued_at)}
            </p>
          ) : null}

          <table className="invoice__table">
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
              {state.invoice.items.map((item, index) => (
                <tr key={`${item.description}-${index}`}>
                  <td data-label="Bezeichnung">{item.description}</td>
                  <td data-label="Menge" className="num">
                    {formatQuantity(item.quantity)}
                  </td>
                  <td data-label="Einzelpreis" className="num">
                    {formatEuro(item.unit_price_cents)}
                  </td>
                  <td data-label="Summe" className="num">
                    {formatEuro(Math.round(item.quantity * item.unit_price_cents))}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>

          <div className="invoice__totals">
            <div className="invoice__total-row">
              <span>Nettobetrag</span>
              <span>{formatEuro(state.invoice.net_cents)}</span>
            </div>
            <div className="invoice__total-row">
              <span>Mehrwertsteuer (19 %)</span>
              <span>{formatEuro(state.invoice.vat_cents)}</span>
            </div>
            <div className="invoice__total-row invoice__total-row--gross">
              <span>Bruttobetrag</span>
              <span>{formatEuro(state.invoice.gross_cents)}</span>
            </div>
          </div>
        </>
      ) : null}
    </section>
  );
}

export default function OrderLookupPage() {
  const [orderNumber, setOrderNumber] = useState("");
  const [plate, setPlate] = useState("");
  const [touched, setTouched] = useState({ orderNumber: false, plate: false });
  const [submitted, setSubmitted] = useState(false);
  const [loading, setLoading] = useState(false);
  const [banner, setBanner] = useState<Banner>(null);
  const [order, setOrder] = useState<PublicOrder | null>(null);
  const [invoiceState, setInvoiceState] = useState<InvoiceState>({ kind: "idle" });

  const formRef = useRef<HTMLFormElement>(null);
  const invoiceRef = useRef<HTMLElement | null>(null);
  const requestId = useRef(0);

  const numberError = !orderNumber.trim() ? "Bitte geben Sie die Auftragsnummer ein." : null;
  const plateError = !plate.trim() ? "Bitte geben Sie das Kennzeichen ein." : null;
  const formValid = !numberError && !plateError;

  useEffect(() => {
    const hash = window.location.hash;
    if (hash === "#rechnung" && invoiceState.kind === "ready") {
      invoiceRef.current?.scrollIntoView?.({ block: "start" });
    }
  }, [invoiceState.kind]);

  const loadInvoice = useCallback(async (nr: string, currentPlate: string, id: number) => {
    setInvoiceState({ kind: "loading" });
    try {
      const response = await apiFetch<InvoiceResponse>(
        `/api/public/orders/${encodeURIComponent(nr)}/invoice?plate=${encodeURIComponent(currentPlate)}`,
      );
      if (requestId.current !== id) {
        return;
      }
      setInvoiceState({ kind: "ready", invoice: response?.invoice ?? null });
    } catch (error) {
      if (requestId.current !== id) {
        return;
      }
      const apiError = error instanceof ApiError ? error : null;
      setInvoiceState({
        kind: "error",
        message: apiError?.message || "Die Rechnung konnte nicht geladen werden.",
        code: apiError?.code,
      });
    }
  }, []);

  const handleSubmit = useCallback(
    async (event: FormEvent<HTMLFormElement>) => {
      event.preventDefault();
      setSubmitted(true);
      setBanner(null);

      if (!formValid) {
        const form = formRef.current;
        const selector = !orderNumber.trim() ? '[name="orderNumber"]' : '[name="plate"]';
        form?.querySelector<HTMLInputElement>(selector)?.focus();
        return;
      }

      const nr = orderNumber.trim().toUpperCase();
      const currentPlate = plate.trim();

      const id = requestId.current + 1;
      requestId.current = id;
      setLoading(true);
      setOrder(null);
      setInvoiceState({ kind: "idle" });

      try {
        const result = await apiFetch<PublicOrder>(
          `/api/public/orders/${encodeURIComponent(nr)}?plate=${encodeURIComponent(currentPlate)}`,
        );
        if (requestId.current !== id) {
          return;
        }
        setOrder(result);
        void loadInvoice(nr, currentPlate, id);
      } catch (error) {
        if (requestId.current !== id) {
          return;
        }
        const apiError = error instanceof ApiError ? error : null;
        if (apiError && apiError.status === 429) {
          setBanner({
            variant: "warning",
            message: apiError.message || RATE_LIMIT_MESSAGE,
            code: apiError.code,
          });
        } else {
          setBanner({
            variant: "error",
            message: apiError?.message || NOT_FOUND_MESSAGE,
            code: apiError?.code,
          });
        }
      } finally {
        if (requestId.current === id) {
          setLoading(false);
        }
      }
    },
    [formValid, orderNumber, plate, loadInvoice],
  );

  return (
    <section className="page page--narrow">
      <header className="page__header">
        <h1 className="page__title">Status abrufen</h1>
        <p className="page__subline">
          Geben Sie Auftragsnummer und Kennzeichen ein, um den Status Ihres Auftrags zu sehen.
        </p>
      </header>

      <div className="card">
        <form className="lookup-form" ref={formRef} onSubmit={handleSubmit} noValidate>
          <div className="lookup-form__grid">
            <FormField
              label="Auftragsnummer"
              name="orderNumber"
              value={orderNumber}
              onChange={(event: ChangeEvent<HTMLInputElement>) =>
                setOrderNumber(event.target.value.toUpperCase())
              }
              onBlur={() => setTouched((state) => ({ ...state, orderNumber: true }))}
              error={numberError}
              touched={touched.orderNumber || submitted}
              required
              autoComplete="off"
              placeholder="AU-ABC123"
            />
            <FormField
              label="Kennzeichen"
              name="plate"
              value={plate}
              onChange={(event: ChangeEvent<HTMLInputElement>) =>
                setPlate(normalizePlate(event.target.value))
              }
              onBlur={() => setTouched((state) => ({ ...state, plate: true }))}
              error={plateError}
              touched={touched.plate || submitted}
              required
              autoComplete="off"
              placeholder="B-AB 1234"
            />
          </div>
          <div className="lookup-form__actions">
            <button
              type="submit"
              className="btn btn--primary"
              disabled={loading}
              aria-busy={loading}
              aria-label={loading ? "Status wird abgerufen" : undefined}
            >
              {loading ? <span className="spinner" aria-hidden="true" /> : "Status abrufen"}
            </button>
          </div>
        </form>
      </div>

      {banner ? (
        <ErrorBanner error={{ code: banner.code, message: banner.message }} variant={banner.variant} />
      ) : null}

      {order ? (
        <section className="card">
          <div className="result__head">
            <div className="result__numbers">
              <span className="result__number">{order.order_number}</span>
              <span className="result__plate">{order.plate}</span>
            </div>
            <StatusBadge status={order.status} />
          </div>

          <dl className="result__meta">
            <div>
              <dt>Wunschtermin</dt>
              <dd>{formatDate(order.desired_date) || "–"}</dd>
            </div>
            <div>
              <dt>Beschreibung</dt>
              <dd>{order.description || "–"}</dd>
            </div>
          </dl>

          <h2 className="subsection-title">Verlauf</h2>
          <StatusTimeline history={order.history ?? []} />
        </section>
      ) : null}

      <InvoiceView state={invoiceState} sectionRef={invoiceRef} />
    </section>
  );
}
