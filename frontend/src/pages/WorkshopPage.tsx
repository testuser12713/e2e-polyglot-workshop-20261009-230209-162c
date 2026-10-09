import { useEffect, useState } from "react";
import ErrorBanner from "../components/ErrorBanner";
import { ApiError, apiFetch } from "../lib/api";
import { formatEuro } from "../lib/format";
import "./WorkshopPage.css";

interface DashboardReport {
  open_orders: number;
  finished_today: number;
  month_revenue_cents: number;
}

function formatCount(value: number): string {
  if (!Number.isFinite(value)) {
    return "0";
  }
  return value.toLocaleString("de-DE");
}

function StatTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="card stat-tile">
      <span className="stat-tile__label">{label}</span>
      <span className="stat-tile__value">{value}</span>
    </div>
  );
}

export default function WorkshopPage() {
  const [report, setReport] = useState<DashboardReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<{ code?: string; message: string } | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setError(null);
    apiFetch<DashboardReport>("/api/reports/dashboard", { signal: controller.signal })
      .then((data) => {
        setReport({
          open_orders: Number(data?.open_orders) || 0,
          finished_today: Number(data?.finished_today) || 0,
          month_revenue_cents: Number(data?.month_revenue_cents) || 0,
        });
        setLoading(false);
      })
      .catch((err: unknown) => {
        if ((err as Error)?.name === "AbortError") {
          return;
        }
        if (err instanceof ApiError) {
          setError({ code: err.code, message: err.message });
        } else {
          setError({ message: "Das Dashboard konnte nicht geladen werden." });
        }
        setLoading(false);
      });
    return () => controller.abort();
  }, []);

  return (
    <section className="page">
      <header className="page__header">
        <h1 className="page__title">Dashboard</h1>
        <p className="page__subline">
          Überblick über offene Aufträge, Tagesabschlüsse und Umsatz.
        </p>
      </header>

      {error ? <ErrorBanner error={error} /> : null}

      {!error && loading ? (
        <div className="stat-grid" aria-busy="true">
          <span className="workshop-loading-text">Wird geladen …</span>
          {[0, 1, 2].map((tile) => (
            <div key={tile} className="card stat-tile stat-tile--skeleton" aria-hidden="true">
              <span className="stat-tile__skeleton-line stat-tile__skeleton-line--label" />
              <span className="stat-tile__skeleton-line stat-tile__skeleton-line--value" />
            </div>
          ))}
        </div>
      ) : null}

      {!error && !loading ? (
        <div className="stat-grid">
          <StatTile label="Offene Aufträge" value={formatCount(report?.open_orders ?? 0)} />
          <StatTile label="Heute fertig" value={formatCount(report?.finished_today ?? 0)} />
          <StatTile
            label="Umsatz laufender Monat"
            value={formatEuro(report?.month_revenue_cents ?? 0)}
          />
        </div>
      ) : null}
    </section>
  );
}
