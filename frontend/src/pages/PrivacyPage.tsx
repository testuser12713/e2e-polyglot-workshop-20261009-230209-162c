import "./Legal.css";

export default function PrivacyPage() {
  return (
    <section className="page page--narrow">
      <header className="page__header">
        <h1 className="page__title">Datenschutzerklärung</h1>
        <p className="page__subline">
          Informationen zur Verarbeitung personenbezogener Daten im Kundenportal der
          Kfz-Werkstatt.
        </p>
      </header>

      <div className="card legal-section">
        <h2 className="card__title">Verantwortlicher</h2>
        <p>
          Verantwortlich für die Datenverarbeitung ist die Kfz-Werkstatt Muster GmbH,
          Werkstattstraße 12, 10115 Berlin, Deutschland, Telefon +49 30 1234567,
          E-Mail kontakt@kfz-werkstatt-muster.example.
        </p>
      </div>

      <div className="card legal-section">
        <h2 className="card__title">Welche Daten wir verarbeiten</h2>
        <p>
          Das Portal verarbeitet ausschließlich die Daten, die für die Abwicklung von
          Terminanfragen und Werkstattaufträgen erforderlich sind:
        </p>
        <ul>
          <li>
            <strong>Kundendaten:</strong> Name, E-Mail-Adresse und Telefonnummer.
          </li>
          <li>
            <strong>Fahrzeugdaten:</strong> Kennzeichen, Marke, Modell und
            Kilometerstand.
          </li>
          <li>
            <strong>Auftragsdaten:</strong> Auftragsnummer, Wunschtermin,
            Problembeschreibung, Statusverlauf, erfasste Positionen sowie die daraus
            erstellte Rechnung.
          </li>
        </ul>
      </div>

      <div className="card legal-section">
        <h2 className="card__title">Zweck der Verarbeitung</h2>
        <p>
          Die Daten werden verwendet, um Ihre Terminanfrage zu bearbeiten, Ihren
          Werkstattauftrag durchzuführen, den Auftragsstatus abrufbar zu machen und
          eine Rechnung zu erstellen. Eine Weitergabe an Dritte zu Werbe- oder
          anderen Zwecken findet nicht statt.
        </p>
      </div>

      <div className="card legal-section">
        <h2 className="card__title">Speicherung</h2>
        <p>
          Alle personenbezogenen Daten werden in der Datenbank des Portals
          (PostgreSQL) gespeichert und ausschließlich dort verarbeitet. Die
          Speicherung erfolgt für die Dauer der Auftragsabwicklung und darüber hinaus
          nur, solange gesetzliche Aufbewahrungspflichten bestehen.
        </p>
      </div>

      <div className="card legal-section">
        <h2 className="card__title">Keine echte E-Mail, keine Drittanbieter</h2>
        <p>
          Kundenbenachrichtigungen werden ausschließlich im Postausgang des Portals
          abgelegt. Es wird <strong>keine echte E-Mail versendet</strong>. Beim Aufruf
          der Seiten werden <strong>keine Ressourcen von Drittanbietern geladen</strong>
          – keine externen Schriftarten, Skripte oder Stylesheets. Es werden keine
          Cookies zu Analyse- oder Werbezwecken eingesetzt.
        </p>
      </div>

      <div className="card legal-section">
        <h2 className="card__title">Ihre Rechte</h2>
        <p>
          Sie haben das Recht auf Auskunft über die zu Ihrer Person gespeicherten
          Daten, auf Berichtigung unrichtiger Daten, auf Löschung, auf Einschränkung
          der Verarbeitung, auf Datenübertragbarkeit sowie das Recht, einer
          Verarbeitung zu widersprechen. Wenden Sie sich dazu an die oben genannten
          Kontaktdaten. Außerdem steht Ihnen ein Beschwerderecht bei einer
          Datenschutz-Aufsichtsbehörde zu.
        </p>
      </div>
    </section>
  );
}
