import { useEffect, useMemo, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import ErrorBanner from "../components/ErrorBanner";
import { ApiError, apiFetch } from "../lib/api";
import { ORDER_STATUSES, statusLabel } from "../lib/format";
import "./WorkshopOrdersPage.css";

interface OrderListItem {
  order_number: string;
  status: string;
  plate: string;
  customer_name: string;
  desired_date: string;
}

const STATUS_BADGE_CLASS: Record<string, string> = {
  requested: "status-badge--requested",
  confirmed: "status-badge--confirmed",
  in_progress: "status-badge--in-progress",
  done: "status-badge--done",
  picked_up: "status-badge--picked-up",
};

function StatusBadge({ status }: { status: string }) {
  const modifier = STATUS_BADGE_CLASS[status] ?? "status-badge--requested";
  return (
    <span className={`status-badge ${modifier}`}>
      <span className="status-badge__dot" aria-hidden="true" />
      {statusLabel(status)}
    </span>
  );
}

function statusOptionLabel(status: string): string {
  const label = statusLabel(status);
  return label.charAt(0).toUpperCase() + label.slice(1);
}

function pad(value: number): string {
  return value.toString().padStart(2, "0");
}

function formatDesiredDate(value: string): string {
  if (!value) {
    return "–";
  }
  const isoDate = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  if (isoDate) {
    return `${isoDate[3]}.${isoDate[2]}.${isoDate[1]}`;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return `${pad(date.getDate())}.${pad(date.getMonth() + 1)}.${date.getFullYear()}`;
}

function SearchIcon() {
  return (
    <svg
      className="search-wrap__icon-svg"
      viewBox="0 0 20 20"
      width="18"
      height="18"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      aria-hidden="true"
    >
      <circle cx="8.5" cy="8.5" r="5.5" />
      <line x1="12.8" y1="12.8" x2="17" y2="17" />
    </svg>
  );
}

export default function WorkshopOrdersPage() {
  const navigate = useNavigate();
  const [status, setStatus] = useState("");
  const [plateInput, setPlateInput] = useState("");
  const [plate, setPlate] = useState("");
  const [orders, setOrders] = useState<OrderListItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<{ code?: string; message: string } | null>(null);

  useEffect(() => {
    const timer = window.setTimeout(() => setPlate(plateInput), 300);
    return () => window.clearTimeout(timer);
  }, [plateInput]);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    const params = new URLSearchParams({ status, plate });
    apiFetch<OrderListItem[]>(`/api/orders?${params.toString()}`, { signal: controller.signal })
      .then((data) => {
        setOrders(Array.isArray(data) ? data : []);
        setLoading(false);
      })
      .catch((err: unknown) => {
        if ((err as Error)?.name === "AbortError") {
          return;
        }
        if (err instanceof ApiError) {
          setError({ code: err.code, message: err.message });
        } else {
          setError({ message: "Die Auftragsliste konnte nicht geladen werden." });
        }
        setLoading(false);
      });
    return () => controller.abort();
  }, [status, plate]);

  const resultLabel = useMemo(() => {
    if (loading) {
      return "Wird geladen …";
    }
    const count = orders.length;
    return `${count} ${count === 1 ? "Auftrag" : "Aufträge"}`;
  }, [loading, orders.length]);

  const openOrder = (orderNumber: string) => {
    navigate(`/werkstatt/auftraege/${orderNumber}`);
  };

  return (
    <section className="page">
      <header className="page__header">
        <h1 className="page__title">Aufträge</h1>
        <p className="page__subline">
          Aufträge nach Status filtern und nach Kennzeichen durchsuchen.
        </p>
      </header>

      <div className="card orders-toolbar">
        <div className="field orders-toolbar__field">
          <label className="field__label" htmlFor="orders-status-filter">
            Status
          </label>
          <div className="select-wrap">
            <select
              id="orders-status-filter"
              name="status"
              className="select"
              value={status}
              onChange={(event) => setStatus(event.target.value)}
            >
              <option value="">Alle Status</option>
              {ORDER_STATUSES.map((value) => (
                <option key={value} value={value}>
                  {statusOptionLabel(value)}
                </option>
              ))}
            </select>
            <span className="select__chevron" aria-hidden="true">
              ▾
            </span>
          </div>
        </div>

        <div className="field orders-toolbar__field">
          <label className="field__label" htmlFor="orders-plate-search">
            Kennzeichen suchen
          </label>
          <div className="search-wrap">
            <span className="search-wrap__icon" aria-hidden="true">
              <SearchIcon />
            </span>
            <input
              id="orders-plate-search"
              name="plate"
              type="text"
              className="field__input search-wrap__input"
              value={plateInput}
              placeholder="z. B. B-AB 1234"
              autoComplete="off"
              onChange={(event) => setPlateInput(event.target.value.toUpperCase())}
            />
            {plateInput ? (
              <button
                type="button"
                className="search-wrap__clear"
                aria-label="Suche zurücksetzen"
                onClick={() => setPlateInput("")}
              >
                ×
              </button>
            ) : null}
            {loading ? <span className="search-wrap__spinner" aria-hidden="true" /> : null}
          </div>
        </div>
      </div>

      {error ? <ErrorBanner error={error} /> : null}

      <p className="orders-result-count" role="status">
        {resultLabel}
      </p>

      {loading ? (
        <div className="card orders-skeleton" aria-busy="true">
          <span className="visually-hidden">Wird geladen …</span>
          {[0, 1, 2, 3, 4].map((row) => (
            <span key={row} className="orders-skeleton__row" aria-hidden="true" />
          ))}
        </div>
      ) : null}

      {!loading && !error && orders.length === 0 ? (
        <div className="card empty-state">
          <p className="empty-state__title">Keine Aufträge gefunden.</p>
          <p className="empty-state__text">Passen Sie Filter oder Suche an.</p>
        </div>
      ) : null}

      {!loading && !error && orders.length > 0 ? (
        <div className="card orders-table-card">
          <table className="orders-table">
            <thead>
              <tr>
                <th scope="col">Auftragsnummer</th>
                <th scope="col">Kunde</th>
                <th scope="col">Kennzeichen</th>
                <th scope="col">Wunschtermin</th>
                <th scope="col">Status</th>
              </tr>
            </thead>
            <tbody>
              {orders.map((order) => (
                <tr
                  key={order.order_number}
                  className="orders-table__row"
                  tabIndex={0}
                  onClick={() => openOrder(order.order_number)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") {
                      openOrder(order.order_number);
                    }
                  }}
                >
                  <td data-label="Auftragsnummer" className="orders-table__mono">
                    <Link
                      to={`/werkstatt/auftraege/${order.order_number}`}
                      onClick={(event) => event.stopPropagation()}
                    >
                      {order.order_number}
                    </Link>
                  </td>
                  <td data-label="Kunde">{order.customer_name}</td>
                  <td data-label="Kennzeichen" className="orders-table__mono">
                    {order.plate}
                  </td>
                  <td data-label="Wunschtermin">{formatDesiredDate(order.desired_date)}</td>
                  <td data-label="Status">
                    <StatusBadge status={order.status} />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </section>
  );
}
