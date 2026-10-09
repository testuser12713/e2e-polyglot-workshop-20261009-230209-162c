import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { apiFetch } from "../lib/api";

type ApiStatus = "loading" | "ok" | "error";

export default function Footer() {
  const [status, setStatus] = useState<ApiStatus>("loading");

  useEffect(() => {
    let active = true;
    const controller = new AbortController();

    apiFetch<{ status: string }>("/healthz", { signal: controller.signal })
      .then((data) => {
        if (active) {
          setStatus(data?.status === "ok" ? "ok" : "error");
        }
      })
      .catch(() => {
        if (active) {
          setStatus("error");
        }
      });

    return () => {
      active = false;
      controller.abort();
    };
  }, []);

  const statusText =
    status === "loading"
      ? "API-Status wird geladen …"
      : status === "ok"
        ? "API verbunden"
        : "API derzeit nicht erreichbar";

  return (
    <footer className="footer">
      <div className="container footer__inner">
        <nav className="footer__links" aria-label="Rechtliches">
          <Link to="/impressum">Impressum</Link>
          <Link to="/datenschutz">Datenschutzerklärung</Link>
        </nav>
        <p
          className={`footer__status footer__status--${status}`}
          role="status"
          aria-live="polite"
        >
          <span className="footer__status-dot" aria-hidden="true" />
          {statusText}
        </p>
      </div>
    </footer>
  );
}
