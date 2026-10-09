import { Link } from "react-router-dom";

export default function HomePage() {
  return (
    <section className="page">
      <header className="page__header">
        <h1 className="page__title">Kfz-Werkstatt Kundenportal</h1>
        <p className="page__subline">
          Termine anfragen, den Status Ihres Auftrags abrufen und Rechnungen einsehen.
        </p>
      </header>
      <div className="card">
        <h2 className="card__title">Was möchten Sie tun?</h2>
        <ul className="link-list">
          <li>
            <Link to="/termin">Termin anfragen</Link>
          </li>
          <li>
            <Link to="/mein-auftrag">Status abrufen</Link>
          </li>
          <li>
            <Link to="/werkstatt/anmelden">Werkstatt-Anmeldung</Link>
          </li>
        </ul>
      </div>
    </section>
  );
}
