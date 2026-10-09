import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "./App";
import ErrorBanner from "./components/ErrorBanner";
import FormField from "./components/FormField";
import { AuthProvider } from "./lib/auth";
import { formatDateTime, formatEuro, statusLabel } from "./lib/format";

function jsonResponse(body: unknown, status = 200, statusText = "OK"): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

function renderApp(path = "/") {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={[path]}>
        <AppRoutes />
      </MemoryRouter>
    </AuthProvider>,
  );
}

beforeEach(() => {
  sessionStorage.clear();
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ status: "ok" })));
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  sessionStorage.clear();
});

describe("app shell", () => {
  it("renders navigation and footer on the home route", () => {
    renderApp("/");
    const nav = screen.getByRole("navigation", { name: "Hauptnavigation" });
    expect(nav).toBeInTheDocument();
    expect(within(nav).getByRole("link", { name: "Termin anfragen" })).toHaveAttribute(
      "href",
      "/termin",
    );
    expect(within(nav).getByRole("link", { name: "Status abrufen" })).toHaveAttribute(
      "href",
      "/mein-auftrag",
    );
    expect(screen.getByRole("link", { name: "Impressum" })).toHaveAttribute(
      "href",
      "/impressum",
    );
    expect(screen.getByRole("link", { name: "Datenschutzerklärung" })).toHaveAttribute(
      "href",
      "/datenschutz",
    );
  });

  it.each([
    ["/termin", "Termin anfragen"],
    ["/mein-auftrag", "Status abrufen"],
    ["/impressum", "Impressum"],
    ["/datenschutz", "Datenschutzerklärung"],
    ["/werkstatt/anmelden", "Werkstatt-Anmeldung"],
  ])("renders the shell and page at %s", (path, heading) => {
    renderApp(path);
    expect(screen.getByRole("heading", { level: 1, name: heading })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Impressum" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Datenschutzerklärung" })).toBeInTheDocument();
  });

  it("keeps the workshop area behind authentication", () => {
    renderApp("/werkstatt");
    expect(
      screen.getByRole("heading", { level: 1, name: "Werkstatt-Anmeldung" }),
    ).toBeInTheDocument();
  });

  it("shows the live API status fetched from the API", async () => {
    renderApp("/");
    await waitFor(() => {
      expect(screen.getByText("API verbunden")).toBeInTheDocument();
    });
    expect(fetch).toHaveBeenCalledWith(
      expect.stringContaining("/healthz"),
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("renders a readable status when the API is unreachable", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new TypeError("Failed to fetch")));
    renderApp("/");
    await waitFor(() => {
      expect(screen.getByText("API derzeit nicht erreichbar")).toBeInTheDocument();
    });
  });
});

describe("FormField", () => {
  it("hides the error until the field was touched", () => {
    const { rerender } = render(
      <FormField
        label="Name"
        name="name"
        value=""
        onChange={() => undefined}
        error="Bitte geben Sie Ihren Namen ein."
        touched={false}
      />,
    );
    expect(screen.queryByText("Bitte geben Sie Ihren Namen ein.")).not.toBeInTheDocument();

    rerender(
      <FormField
        label="Name"
        name="name"
        value=""
        onChange={() => undefined}
        error="Bitte geben Sie Ihren Namen ein."
        touched={true}
      />,
    );
    expect(screen.getByText("Bitte geben Sie Ihren Namen ein.")).toBeInTheDocument();
  });

  it("sets aria-invalid only when the error is shown", () => {
    render(
      <FormField
        label="E-Mail"
        name="email"
        value="x"
        onChange={() => undefined}
        error="Bitte geben Sie eine gültige E-Mail-Adresse ein."
        touched={true}
      />,
    );
    expect(screen.getByLabelText(/E-Mail/)).toHaveAttribute("aria-invalid", "true");
  });
});

describe("ErrorBanner", () => {
  it("shows the API message verbatim and keeps the stable code", () => {
    render(<ErrorBanner error={{ code: "invalid_credentials", message: "E-Mail oder Passwort ist falsch." }} />);
    const banner = screen.getByRole("alert");
    expect(banner).toHaveAttribute("data-error-code", "invalid_credentials");
    expect(screen.getByText("E-Mail oder Passwort ist falsch.")).toBeInTheDocument();
  });
});

describe("format helpers", () => {
  it("maps order status values to the contracted German labels", () => {
    expect(statusLabel("requested")).toBe("angefragt");
    expect(statusLabel("confirmed")).toBe("bestätigt");
    expect(statusLabel("in_progress")).toBe("in Arbeit");
    expect(statusLabel("done")).toBe("fertig");
    expect(statusLabel("picked_up")).toBe("abgeholt");
  });

  it("formats integer cents as German currency", () => {
    expect(formatEuro(1248000)).toBe("12.480,00\u00A0€");
    expect(formatEuro(0)).toBe("0,00\u00A0€");
    expect(formatEuro(22250)).toBe("222,50\u00A0€");
  });

  it("formats ISO timestamps in German date/time format", () => {
    const iso = new Date(2025, 2, 7, 9, 14).toISOString();
    expect(formatDateTime(iso)).toBe("07.03.2025, 09:14 Uhr");
  });
});

describe("mobile navigation", () => {
  it("toggles the menu and reports state to assistive tech", async () => {
    const user = userEvent.setup();
    renderApp("/");
    const toggle = screen.getByRole("button", { name: "Menü öffnen" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    await user.click(toggle);
    expect(screen.getByRole("button", { name: "Menü schließen" })).toHaveAttribute(
      "aria-expanded",
      "true",
    );
  });
});
