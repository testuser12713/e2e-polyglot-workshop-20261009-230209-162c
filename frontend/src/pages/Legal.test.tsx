import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "../App";
import { AuthProvider } from "../lib/auth";

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

function footerNav() {
  return screen.getByRole("navigation", { name: "Rechtliches" });
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

describe("legal pages", () => {
  it("offers the Impressum and Datenschutz links in the footer of every page", () => {
    for (const path of ["/", "/termin", "/mein-auftrag", "/werkstatt/anmelden"]) {
      const { unmount } = renderApp(path);
      const nav = within(footerNav());
      expect(nav.getByRole("link", { name: "Impressum" })).toHaveAttribute(
        "href",
        "/impressum",
      );
      expect(nav.getByRole("link", { name: "Datenschutzerklärung" })).toHaveAttribute(
        "href",
        "/datenschutz",
      );
      unmount();
    }
  });

  it("renders the Impressum with provider, address and contact details", () => {
    renderApp("/impressum");
    expect(screen.getByRole("heading", { level: 1, name: "Impressum" })).toBeInTheDocument();
    expect(screen.getByText("Kfz-Werkstatt Muster GmbH")).toBeInTheDocument();
    expect(screen.getByText(/Werkstattstraße 12, 10115 Berlin/)).toBeInTheDocument();
    expect(screen.getByText("+49 30 1234567")).toBeInTheDocument();
    expect(
      screen.getByText("kontakt@kfz-werkstatt-muster.example"),
    ).toBeInTheDocument();
  });

  it("renders the Datenschutzerklärung with data, purpose, storage and rights", () => {
    renderApp("/datenschutz");
    expect(
      screen.getByRole("heading", { level: 1, name: "Datenschutzerklärung" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/Kundendaten:/)).toBeInTheDocument();
    expect(screen.getByText(/Fahrzeugdaten:/)).toBeInTheDocument();
    expect(screen.getByText(/Auftragsdaten:/)).toBeInTheDocument();
    expect(screen.getByText(/Zweck der Verarbeitung/)).toBeInTheDocument();
    expect(screen.getByText(/PostgreSQL/)).toBeInTheDocument();
    expect(screen.getByText(/keine echte E-Mail versendet/)).toBeInTheDocument();
    expect(
      screen.getByText(/keine Ressourcen von Drittanbietern geladen/),
    ).toBeInTheDocument();
    expect(screen.getByText(/Ihre Rechte/)).toBeInTheDocument();
  });

  it("navigates to the Impressum from the footer link", async () => {
    const user = userEvent.setup();
    renderApp("/");
    await user.click(within(footerNav()).getByRole("link", { name: "Impressum" }));
    expect(screen.getByRole("heading", { level: 1, name: "Impressum" })).toBeInTheDocument();
  });

  it("navigates to the Datenschutzerklärung from the footer link", async () => {
    const user = userEvent.setup();
    renderApp("/");
    await user.click(
      within(footerNav()).getByRole("link", { name: "Datenschutzerklärung" }),
    );
    expect(
      screen.getByRole("heading", { level: 1, name: "Datenschutzerklärung" }),
    ).toBeInTheDocument();
  });
});
