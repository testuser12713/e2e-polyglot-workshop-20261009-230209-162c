# Kfz-Werkstatt-Kundenportal

Ein Kundenportal für eine Kfz-Werkstatt als Drei-Dienste-System: eine Werkstatt-API
in Go, ein Rechnungs-Worker in Python und eine Web-App mit Vite, React und
TypeScript. Kundinnen und Kunden fragen online einen Termin an, rufen den Status
ihres Auftrags ab und sehen die fertige Rechnung ein. Die Werkstatt verwaltet nach
der Anmeldung Kunden, Fahrzeuge und Aufträge, erfasst Positionen aus Arbeitszeit
und Teilen, wechselt den Auftragsstatus und sieht ein Dashboard.

## Tech Stack

- **API**: Go mit `net/http` aus der Standardbibliothek, `pgx` als
  PostgreSQL-Treiber, `go-redis` als Valkey-Client.
- **Worker**: Python mit Valkey-Client, Konfiguration über Umgebungsvariablen.
- **Web-App**: Vite mit React und TypeScript.
- **Datenbank**: PostgreSQL 18.
- **Warteschlange**: Valkey 9.1.
- **Tests**: `go test` mit `net/http/httptest` gegen ein echtes PostgreSQL,
  `pytest` für den Worker, Vitest mit Testing Library für das Frontend.

## Voraussetzungen

- Go (aktuelle Version, getestet mit 1.27)
- Python 3 (für den Worker)
- Node.js und npm (für die Web-App)
- Docker oder eine andere Möglichkeit, PostgreSQL 18 und Valkey 9.1 zu starten

## Datenbanken starten

Das Repository enthält eine `compose.yaml`, die genau die Datenbanken startet, auf
denen das Produkt läuft. Aus dem Projektverzeichnis:

```bash
docker compose up -d
```

Das startet PostgreSQL auf Port `5432` und Valkey auf Port `6379`. Die
Zugangsdaten der lokalen Entwicklungsumgebung sind `app` / `app` und können über
die Variablen `POSTGRES_USER`, `POSTGRES_PASSWORD` und `POSTGRES_DB` überschrieben
werden.

Die API legt ihr Schema beim Start selbst an (`backend/migrations/0001_init.sql`,
idempotent), es ist also keine manuelle Migration nötig.

## Konfiguration

Die API liest ihre Startkonfiguration aus der Umgebung. Eine vollständige Vorlage
liegt als `.env.example` im Projektverzeichnis:

```bash
cp .env.example .env
# .env an die eigene Umgebung anpassen, z. B. EMPLOYEE_BOOTSTRAP_PASSWORD setzen
set -a && . ./.env && set +a
```

| Variable | Pflicht | Bedeutung |
| --- | --- | --- |
| `DATABASE_URL` | ja | Verbindung zu PostgreSQL, z. B. `postgresql://app:app@127.0.0.1:5432/app` |
| `VALKEY_URL` | ja | Verbindung zu Valkey, z. B. `redis://127.0.0.1:6379/0` |
| `WEB_ORIGIN` | nein | Ursprung der Web-App für CORS (Vorgabe `http://localhost:5173`) |
| `API_PORT` | nein | Port der API (Vorgabe `8080`) |
| `EMPLOYEE_BOOTSTRAP_EMAIL` | nein | E-Mail des Bootstrap-Mitarbeiters |
| `EMPLOYEE_BOOTSTRAP_PASSWORD` | nein | Passwort des Bootstrap-Mitarbeiters (nur als Hash in der Datenbank, nie im Log) |

Geheimnisse werden nie im Repository abgelegt: `EMPLOYEE_BOOTSTRAP_PASSWORD` und
die Datenbank-Zugangsdaten kommen ausschließlich aus der Startkonfiguration.

## Ausführen in der Entwicklung

```bash
cd backend
go run .
```

Die API ist danach unter `http://localhost:8080` erreichbar (bzw. unter dem in
`API_PORT` gesetzten Port). Der Health-Check ist:

```bash
curl http://localhost:8080/healthz
# {"status":"ok"}
```

## Produktions-Build

Die API wird als eigenständige Binärdatei gebaut:

```bash
cd backend
go build -o bin/api .
./bin/api
```

Die Web-App wird statisch gebaut und kann von einem beliebigen Webserver
ausgeliefert werden:

```bash
cd frontend
npm ci
npm run build
```

## Tests

```bash
# API (gegen echtes PostgreSQL und Valkey; beide müssen laufen)
cd backend
go test ./...

# Worker
cd worker
PYTHONPATH=. python3 -m pytest

# Web-App
cd frontend
npm test
npm run build
```

## So wird das Produkt benutzt

### Öffentlicher Kundenbereich (ohne Anmeldung)

- **Termin anfragen** — Name, E-Mail, Telefon, Kennzeichen, Marke, Modell,
  Kilometerstand, Wunschtermin und Problembeschreibung eingeben; nach dem
  Absenden erscheint die vergebene Auftragsnummer.
- **Status abrufen** — Auftragsnummer und Kennzeichen eingeben, um den aktuellen
  Status samt Statusverlauf zu sehen.
- **Rechnung ansehen** — zur selben Kombination die fertige Rechnung mit
  Positionen, Nettobetrag, 19 % Mehrwertsteuer und Bruttobetrag einsehen; vorher
  erscheint der Hinweis, dass die Rechnung noch nicht vorliegt.

### Werkstattbereich (nach Anmeldung)

- **Anmelden** mit der konfigurierten E-Mail und dem konfigurierten Passwort.
- **Dashboard** mit offenen Aufträgen, heute fertig gewordenen Aufträgen und dem
  Umsatz des laufenden Monats.
- **Auftragsliste** mit Filter nach Status und Suche nach Kennzeichen.
- **Auftragsdetails** mit Bestätigen, Erfassen von Positionen (Arbeitszeit oder
  Teil) und dem jeweils nächsten erlaubten Statuswechsel.

## API-Endpunkte

Jede Fehlerantwort hat denselben Aufbau mit stabilem Fehlercode und lesbarer
Meldung:

```json
{ "error": { "code": "not_found", "message": "Auftrag nicht gefunden." } }
```

Geschützte Endpunkte erwarten den Header `Authorization: Bearer <token>`.

### Öffentlich

| Methode | Pfad | Body | Erfolg |
| --- | --- | --- | --- |
| `GET` | `/healthz` | — | `200 {"status":"ok"}` |
| `POST` | `/api/orders` | `{"name","email","phone","plate","brand","model","mileage_km","desired_date","description"}` | `201 {"order_number","status"}` oder `409 plate_taken` |
| `GET` | `/api/public/orders/{nr}?plate=X` | — | `200 {"order_number","status","desired_date","description","plate","history"}` oder `404` |
| `GET` | `/api/public/orders/{nr}/invoice?plate=X` | — | `200 {"invoice":<invoice>\|null}` oder `404` |
| `POST` | `/api/auth/login` | `{"email","password"}` | `200 {"token","employee":{"id","email","name"}}` oder `401 invalid_credentials` |

### Geschützt (Bearer)

| Methode | Pfad | Body | Erfolg |
| --- | --- | --- | --- |
| `POST` | `/api/auth/logout` | — | `204` |
| `POST` | `/api/customers` | `{"name","email","phone"}` | `201 customer` |
| `GET` | `/api/customers/{id}` | — | `200 customer` oder `404` |
| `POST` | `/api/vehicles` | `{"customer_id","plate","brand","model","mileage_km"}` | `201 vehicle` oder `409 plate_taken` |
| `GET` | `/api/vehicles?plate=X` | — | `200 [vehicle]` |
| `GET` | `/api/orders?status=&plate=` | — | `200 [{order_number,status,plate,customer_name,desired_date}]` |
| `GET` | `/api/orders/{nr}` | — | `200 {order_number,status,desired_date,description,customer,vehicle,items,history,invoice}` |
| `POST` | `/api/orders/{nr}/status` | `{"status"}` | `200 {order_number,status,history}` oder `409 invalid_transition` |
| `POST` | `/api/orders/{nr}/items` | `{"kind","description","hours?","quantity?","unit_price_cents?"}` | `201 item` |
| `GET` | `/api/reports/dashboard` | — | `200 {open_orders,finished_today,month_revenue_cents}` |

### Grenzen

- `POST /api/auth/login`: 10 Anfragen pro Client-IP und Minute, danach `429 too_many_requests`.
- `GET /api/public/*`: 20 Anfragen pro Client-IP und Minute, danach `429 too_many_requests`.

### Datentypen

- `customer` = `{id, name, email, phone}`
- `vehicle` = `{id, customer_id, plate, brand, model, mileage_km}`
- `item` = `{id, kind: "labor"|"part", description, hours|null, quantity|null, unit_price_cents|null}`
- `history` = `[{status, changed_at}]`
- `invoice` = `{invoice_number, issued_at, net_cents, vat_cents, gross_cents, items:[{description, quantity, unit_price_cents}]}`

Auftragsstatus in genau dieser Reihenfolge: `requested`, `confirmed`,
`in_progress`, `done`, `picked_up`. Geldbeträge sind immer ganzzahlige Cent in
Feldern mit der Endung `_cents`.

## Funktionen

- Kunden mit Name, E-Mail und Telefon anlegen und abrufen.
- Fahrzeuge einem Kunden zuordnen, Kennzeichen eindeutig.
- Aufträge mit Fahrzeug, Wunschtermin und Problembeschreibung anfragen.
- Statuslauf `angefragt → bestätigt → in Arbeit → fertig → abgeholt` mit
  protokolliertem Zeitpunkt je Wechsel.
- Positionen als Arbeitszeit in Stunden oder als Teil mit Menge und Einzelpreis.
- Beim Status `fertig` gelangt genau eine Nachricht mit der Auftragsnummer in die
  Valkey-Warteschlange `invoices`.
- Der Python-Worker erzeugt daraus eine Rechnung (netto, 19 % MwSt., brutto) und
  eine Benachrichtigung im Postausgang.
- Öffentliche Terminanfrage, Status- und Rechnungsabruf mit Auftragsnummer und
  Kennzeichen.
- Werkstattbereich mit Anmeldung, Auftragsliste, Filter, Suche, Positionserfassung,
  Statuswechsel und Dashboard.
