# Design — Project Identity

> This document is project-long-lived. Tokens are not changed without
> the Architect's approval. Developers MUST use these tokens
> instead of improvising their own colors/spacings.

## Style Direction

Calm, professional service portal: light neutral surfaces with a single deep petrol accent (#0F6E8C), status-driven color used only for order states, self-hosted system fonts only, dense but airy tables for the workshop and large, forgiving touch targets for the customer area — closer to Stripe's restraint than to a marketing site.

## Colors

- `--color-bg`: **#F6F7F9**
- `--color-surface`: **#FFFFFF**
- `--color-surfaceMuted`: **#EEF1F4**
- `--color-fg`: **#1B2430**
- `--color-fgMuted`: **#5A6675**
- `--color-border`: **#DDE2E8**
- `--color-borderStrong`: **#C3CBD4**
- `--color-accent`: **#0F6E8C**
- `--color-accentHover`: **#0C5A73**
- `--color-accentActive`: **#0A4C61**
- `--color-accentSubtle`: **#E6F2F6**
- `--color-fgOnAccent`: **#FFFFFF**
- `--color-navBg`: **#12303C**
- `--color-navFg`: **#E8EEF1**
- `--color-navFgMuted`: **#9FB4BE**
- `--color-danger`: **#B3261E**
- `--color-dangerSubtle`: **#FBEAE8**
- `--color-success`: **#1E7A46**
- `--color-successSubtle`: **#E6F4EC**
- `--color-warning`: **#8A5A00**
- `--color-warningSubtle`: **#FBF1DE**
- `--color-focusRing`: **#0F6E8C**
- `--color-overlay`: **rgba(18, 48, 60, 0.5)**
- `--color-statusAngfragtBg`: **#EEF1F4**
- `--color-statusAngfragtFg`: **#5A6675**
- `--color-statusBestaetigtBg`: **#E6F2F6**
- `--color-statusBestaetigtFg`: **#0F6E8C**
- `--color-statusInArbeitBg`: **#FBF1DE**
- `--color-statusInArbeitFg`: **#8A5A00**
- `--color-statusFertigBg`: **#E6F4EC**
- `--color-statusFertigFg`: **#1E7A46**
- `--color-statusAbgeholtBg`: **#E3E7EC**
- `--color-statusAbgeholtFg`: **#3C4756**

## Typography

- `font_family`: system-ui, -apple-system, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, 'Noto Sans', sans-serif
- `font_mono`: ui-monospace, SFMono-Regular, Menlo, Consolas, 'DejaVu Sans Mono', monospace
- `heading_weight`: 600
- `body_weight`: 400
- `size_xs`: 13px
- `size_sm`: 14px
- `size_md`: 16px
- `size_lg`: 20px
- `size_xl`: 26px
- `size_2xl`: 34px
- `line_height_body`: 1.55
- `line_height_heading`: 1.25
- `numeric_feature`: font-variant-numeric: tabular-nums for all prices, hours, quantities and dashboard figures

## Spacing Scale

- `--space-0`: 4px
- `--space-1`: 8px
- `--space-2`: 12px
- `--space-3`: 16px
- `--space-4`: 24px
- `--space-5`: 32px
- `--space-6`: 48px

## Border-Radii

- `--radius-sm`: 6px
- `--radius-md`: 10px
- `--radius-lg`: 14px
- `--radius-pill`: 999px

## Components

### Button

Variants: primary (bg=accent, fg=fgOnAccent), secondary (bg=surface, fg=accent, border 1px borderStrong), ghost (transparent, fg=fgMuted), danger (bg=danger, fg=fgOnAccent). Sizing: min-height 44px and min-width 44px on every breakpoint (workshop on a phone), padding 10px 20px, radius md, font 15px weight 600. States: hover bg=accentHover (secondary/ghost: bg=accentSubtle); active bg=accentActive plus translateY(1px); focus-visible 2px focusRing outline with 2px offset, never removed; disabled opacity 0.5, cursor not-allowed, aria-disabled="true", no hover/active change. Busy: label replaced by spinner plus the same width (no layout shift), aria-busy. Label examples: 'Termin anfragen', 'Status abrufen', 'Anmelden', 'Position hinzufügen', 'Bestätigen'. Icon-only buttons are 44x44 with aria-label.

### TextField

Label above (13px weight 600, fg), input below: height 44px, padding 10px 12px, border 1px border, radius sm, bg surface, 16px font (no iOS zoom), full width of its column. Placeholder is a hint only, never the label. Focus: border accent plus 2px focusRing ring. Error: border danger, message 13px danger below the field, aria-describedby, aria-invalid. Read-only/disabled: bg surfaceMuted, fgMuted. Used for Name, E-Mail, Telefon, Kennzeichen, Marke, Modell, Kilometerstand, Problembeschreibung.

### FormValidation

Validation only after blur of the touched field or after submit; an untouched form never shows an error (AC-21). On submit focus jumps to the first invalid field. Blur messages in German, field-specific: 'Bitte geben Sie Ihren Namen ein.', 'Bitte geben Sie eine gültige E-Mail-Adresse ein.', 'Bitte geben Sie ein gültiges Kennzeichen ein.', 'Bitte wählen Sie einen Wunschtermin in der Zukunft.', 'Bitte geben Sie eine Problembeschreibung ein.' On submit, one summary alert above the form: 'Bitte prüfen Sie die markierten Felder.' No error styling on fields the user never touched.

### Select

Native select, height 44px, padding 10px 36px 10px 12px, border 1px border, radius sm, bg surface, custom chevron in fgMuted, 16px font. Used for the Statusfilter: 'Alle Status', 'Angefragt', 'Bestätigt', 'In Arbeit', 'Fertig', 'Abgeholt'. Focus ring identical to TextField.

### SearchField

Companion to the status filter in the workshop order list: TextField with magnifier icon, label 'Kennzeichen suchen', input uppercased on entry, debounce 300ms, clear button 44x44 with aria-label 'Suche zurücksetzen'. While loading shows an inline spinner at the right edge, the result count stays visible to avoid flicker.

### DateTimeField

Wish date/time input for the appointment request: height 44px like TextField, calendar/clock icon, displayed in German format DD.MM.YYYY HH:MM, stored and transmitted as ISO 8601. Min value is now; no availability check (out of scope), so any future slot is accepted and only the format is validated.

### Card

bg surface, border 1px border, radius lg, padding 24px (16px on width <480px), shadow 0 1px 2px rgba(18,48,60,0.06). Title 20px weight 600, optional subtitle 14px fgMuted. Used for 'Termin anfragen', status result, invoice, dashboard tiles and the order detail in the workshop.

### StatusBadge

Pill (radius pill), padding 4px 10px, 13px weight 600, always dot + German label, never color alone (color-blind safe). Colors per status: Angefragt statusAngfragtBg/Fg, Bestätigt statusBestaetigtBg/Fg, In Arbeit statusInArbeitBg/Fg, Fertig statusFertigBg/Fg, Abgeholt statusAbgeholtBg/Fg. The badge is read-only; the state change happens only through the Button that names the target state, e.g. 'Auf „In Arbeit" setzen'.

### StatusTimeline

Order history in the customer area and order detail: vertical line (1px border) with one entry per performed transition; each entry shows the reached state as StatusBadge and the timestamp as '07.03.2025, 09:14 Uhr' in fgMuted 13px with tabular-nums. Newest entry on top. Empty case: 'Noch keine Statuswechsel.'

### OrderTable

Workshop order list. >=768px: table, header 13px weight 600 fgMuted, rows 48px min height, 1px border between rows, hover bg surfaceMuted, whole row clickable (keyboard: Enter on focused row). Columns: Auftragsnummer, Kennzeichen, Fahrzeug (Marke Modell), Kunde, Wunschtermin, Status. Auftragsnummer and Kennzeichen in font_mono. <768px: same data as stacked cards, label/value pairs, no horizontal scrolling (AC-19). Sorting by Wunschtermin desc by default, sortable headers with aria-sort.

### DashboardStatTile

Three equal tiles inside the card grid: 'Offene Aufträge', 'Heute fertig', 'Umsatz laufender Monat'. Label 13px fgMuted above, value 34px weight 600 tabular-nums below; money in German format '12.480,00 €', counts without decimal separator. Empty value is '0' or '0,00 €', never a dash placeholder. Tiles are informational only and carry no click affordance (AC-20).

### InvoiceView

Card with heading 'Rechnung' and number 'Rechnung Nr. AU-2025-000123'. Line item table: Bezeichnung, Menge ('2,5 h', '1'), Einzelpreis ('89,00 €'), Summe ('222,50 €'); numeric columns right-aligned with tabular-nums, on <768px rows become label/value blocks. Totals block right-aligned with a 1px border top: 'Nettobetrag', 'Mehrwertsteuer (19 %)', 'Bruttobetrag' (weight 600, 20px). All amounts rendered from integer cents, never reformatted as floats. States: loading = 3 skeleton rows; not yet available = 'Die Rechnung liegt noch nicht vor.' plus subline 'Sie erscheint hier, sobald der Auftrag fertiggestellt ist.'; error = InlineAlert with the API message. No PDF/download button (out of scope).

### LookupForm

Customer status and invoice lookup: two fields side by side on >=768px, stacked below — 'Auftragsnummer' and 'Kennzeichen' — plus primary Button 'Status abrufen'. Kennzeichen is uppercased and stripped of separators while typing. Exactly one error message for a wrong combination: 'Zu dieser Kombination aus Auftragsnummer und Kennzeichen wurde kein Auftrag gefunden.' HTTP 429 renders warning styling and 'Zu viele Versuche. Bitte versuchen Sie es in einer Minute erneut.' with the retry hint, never a blank page (AC-11, AC-25, AC-31). On success the result card shows StatusBadge, StatusTimeline and the invoice section.

### LoginForm

Workshop login: TextField 'E-Mail' (type=email, autocapitalize off) and 'Passwort' (type=password with a 44x44 show/hide toggle, aria-label 'Passwort anzeigen'), primary Button 'Anmelden' full width on <480px. One generic failure message for both cases: 'E-Mail oder Passwort ist falsch.' (no user enumeration). HTTP 429: warning InlineAlert 'Zu viele Anmeldeversuche. Bitte warten Sie eine Minute.' The workshop area is only reachable with a valid session; without it the app immediately renders the login view (AC-14).

### PositionForm

Add line items to an order in the workshop area. A segmented control (two 44px-high tabs) chooses the type: 'Arbeitszeit' or 'Teil'. Arbeitszeit: 'Stunden' (decimal with comma, e.g. '2,5'), validates > 0 and max 2 decimals. Teil: 'Bezeichnung', 'Menge' (whole number >= 1), 'Einzelpreis' (decimal input with '€' suffix, converted to whole cents on submit). Primary Button 'Position hinzufügen'; on success the new position appears in the list and the form is cleared, with a success toast 'Position hinzugefügt.'. Errors keep the entered values so nothing is retyped.

### StatusActionBar

Sticky at the bottom of the order detail on <768px (shell-bottom, safe-area padding) and inline in the header on desktop: one primary Button naming exactly the next allowed state — 'Auftrag bestätigen', 'In Arbeit setzen', 'Als fertig melden', 'Als abgeholt markieren'. Only the allowed transition has a button; impossible transitions are not rendered at all (no fake disabled buttons, AC-20). Each transition opens a Modal for confirmation because a wrong status change cannot be undone (out of scope), with the target status in plain language.

### Modal

Centered dialog, max-width 560px (720px for the invoice view), radius lg, padding 24px, shadow 0 8px 24px rgba(18,48,60,0.16), backdrop overlay. Title 20px weight 600, action row right-aligned (secondary 'Abbrechen' + primary/danger confirm), actions stacked full width on <480px. ESC and backdrop close, focus trapped inside, focus returned to the trigger. Confirmation texts name the target state, e.g. 'Auftrag auf „Fertig" setzen? Danach wird automatisch eine Rechnung erstellt.'

### InlineAlert

Variants info/success/warning/error: bg surfaceMuted/accentSubtle/warningSubtle/dangerSubtle, left border 3px in the matching strong color, radius sm, padding 12px 16px, 14px text, optional close button 44x44. role="status" for success/info, role="alert" for error/warning. API error messages are shown verbatim from the error body (stable code hidden in a data attribute, readable message visible), never swallowed (AC-18). Examples: 'Auftrag bestätigt.', 'Dieser Statuswechsel ist nicht erlaubt.'

### EmptyState

Centered block, padding 48px 24px: muted icon, heading 16px weight 600, one explanatory sentence 14px fgMuted, at most one primary action that leads somewhere real. Text examples: 'Keine Aufträge gefunden.' plus 'Passen Sie Filter oder Suche an.'; 'Noch keine Fahrzeuge erfasst.'; 'Die Rechnung liegt noch nicht vor.' Never a disabled or decorative button.

### Skeleton

Loading placeholder: blocks in surfaceMuted, radius sm, 1.2s pulse opacity 1 → 0.6, no layout shift when real content arrives; used for order list (5 rows), dashboard tiles (3) and invoice (3 line rows). Every loading state additionally carries aria-busy="true" and a hidden 'Wird geladen …' text.

### AppShell

Top bar 64px (56px on <480px), bg navBg, fg navFg, product name left in 16px weight 600. Public customer area: navigation as three tabs 'Termin anfragen', 'Status abrufen', 'Rechnung ansehen' (44px height, active item marked by an accent underline plus a subtle accentSubtle background). Workshop area after login: same bar plus nav 'Dashboard', 'Aufträge' and on the right the employee e-mail with a 'Abmelden' button; active nav item fg=navFg with 3px accent underline. <768px: hamburger (44x44, aria-expanded) opens a drawer with the same items stacked in 48px rows. Visible skip link 'Zum Inhalt springen' on focus.

### PageHeader

Above every page's content: h1 26px weight 600 (34px on >=768px), optional 14px fgMuted subline explaining what the page does, e.g. 'Geben Sie Auftragsnummer und Kennzeichen ein, um den Status Ihres Auftrags zu sehen.'. Wraps to multiple lines without truncation on 320px width.

### Footer

On every page, incl. login: bg bg, border-top 1px border, padding 24px, 13px fgMuted, links 'Impressum' and 'Datenschutzerklärung' reachable from every screen (AC-28). Links are real anchors with visited/hover underline and a focus ring; on <480px they stack vertically with 44px tap height. No third-party embed, font, script or stylesheet anywhere (AC-29).

## Layout Principles

- Container max-width 1200px with 24px side padding (16px below 768px, 12px below 480px); forms, the customer lookup and the invoice use a narrower 720px column so lines stay readable.
- Breakpoints: 480px (small phone), 768px (tablet — tables become card lists, sidebar becomes drawer), 1024px (desktop — order detail with side-by-side detail and action bar), 1280px+ (content stays centered, never stretches beyond 1200px).
- Grid/flex: dashboard 3 tiles in a grid of 1 column below 768px, 3 columns from 768px; order list plus detail as 1 column below 1024px and 5fr/7fr split from 1024px; forms are single-column stacking, only Auftragsnummer and Kennzeichen share a row from 768px.
- Vertical rhythm: 8px grid; 8px between label and input, 16px between fields, 24px between cards, 32–48px between page sections. Section titles 20px weight 600 with 12px top margin.
- ONE format for every value in the product (no page invents its own): money always German with thousands separator, two decimals and a non-breaking space before the euro sign — '12.480,00 €' — rendered from integer cents, never from floats; VAT rate as '19 %'; dates as '07.03.2025'; timestamps of status changes and invoices as '07.03.2025, 09:14 Uhr'; working time as '2,5 h' (comma, one decimal, space before h); part quantity as a whole number; percentages and counts without decimals. All numbers use tabular-nums so columns line up.
- Auftragsnummer and Kennzeichen are always displayed in the monospace stack and never wrapped or truncated (Kennzeichen uppercased), because customers compare them character by character.
- Touch and reachability: every interactive target is at least 44x44px; primary actions on the customer pages are at least 44px high and full width below 480px; the status action bar in the workshop order detail sticks to the bottom of the viewport on <768px with safe-area padding.
- Accessibility: one h1 per page, headings descending without gaps, text contrast at least 4.5:1 against the surface (fg #1B2430 on #FFFFFF, fgOnAccent #FFFFFF on accent #0F6E8C), never color as the only signal (StatusBadge always carries text, errors carry an icon and a message), visible focus ring on every focusable element, no horizontal scrolling from 320px up (AC-19).
- No third-party resources: no external fonts, icons, scripts or stylesheets; the system font stack is the only type used, icons ship as inline SVG from the product itself (AC-29).
- Empty, loading and error states are designed as first-class screens, not leftovers: skeletons for loading, EmptyState with one real action for nothing-found, InlineAlert with the API's own readable message for errors — no blank or crashing page anywhere (AC-11, AC-18, AC-20).
