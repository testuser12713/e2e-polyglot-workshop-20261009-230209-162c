import "./Legal.css";

export default function ImprintPage() {
  return (
    <section className="page page--narrow">
      <header className="page__header">
        <h1 className="page__title">Impressum</h1>
        <p className="page__subline">
          Anbieterkennzeichnung für das Kundenportal der Kfz-Werkstatt.
        </p>
      </header>

      <div className="card">
        <h2 className="card__title">Angaben gemäß § 5 DDG</h2>
        <dl className="legal-facts">
          <dt>Anbieter</dt>
          <dd>Kfz-Werkstatt Muster GmbH</dd>
          <dt>Anschrift</dt>
          <dd>Werkstattstraße 12, 10115 Berlin, Deutschland</dd>
          <dt>Telefon</dt>
          <dd>+49 30 1234567</dd>
          <dt>E-Mail</dt>
          <dd>kontakt@kfz-werkstatt-muster.example</dd>
          <dt>Vertretungsberechtigt</dt>
          <dd>Geschäftsführer Max Mustermann</dd>
          <dt>Handelsregister</dt>
          <dd>Amtsgericht Berlin, HRB 123456</dd>
          <dt>Umsatzsteuer-Identifikationsnummer</dt>
          <dd>DE123456789</dd>
        </dl>
      </div>

      <div className="card">
        <h2 className="card__title">Kontakt</h2>
        <p>
          Sie erreichen uns telefonisch während der Öffnungszeiten unter der oben
          genannten Nummer oder jederzeit per E-Mail. Die Kontaktdaten dienen der
          Abwicklung von Terminanfragen und Werkstattaufträgen.
        </p>
      </div>

      <div className="card">
        <h2 className="card__title">Haftung für Inhalte und Links</h2>
        <p>
          Als Diensteanbieter sind wir für eigene Inhalte auf diesen Seiten nach den
          allgemeinen Gesetzen verantwortlich. Für Inhalte externer Links sind
          ausschließlich deren Betreiber verantwortlich. Dieses Portal bindet keine
          fremden Ressourcen ein; alle Inhalte werden von uns selbst ausgeliefert.
        </p>
      </div>
    </section>
  );
}
