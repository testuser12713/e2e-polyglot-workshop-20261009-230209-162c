import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useEffect } from "react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { AppRoutes } from "../App";
import RequireAuth from "../components/RequireAuth";
import { apiFetch } from "../lib/api";
import { AuthProvider } from "../lib/auth";
import LoginPage from "./LoginPage";

function jsonResponse(body: unknown, status = 200, statusText = "OK"): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

interface RouteStub {
  match: string;
  body: unknown;
  status?: number;
}

function stubFetch(routes: RouteStub[]): void {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL) => {
      const url = typeof input === "string" ? input : input.toString();
      const route = routes.find((entry) => url.includes(entry.match));
      if (!route) {
        return jsonResponse({}, 404, "Not Found");
      }
      return jsonResponse(route.body, route.status ?? 200);
    }),
  );
}

const EMPLOYEE = { id: 1, email: "anna@werkstatt.de", name: "Anna" };

function renderApp(path = "/werkstatt/anmelden") {
  return render(
    <AuthProvider>
      <MemoryRouter initialEntries={[path]}>
        <AppRoutes />
      </MemoryRouter>
    </AuthProvider>,
  );
}

async function submitLogin(user: ReturnType<typeof userEvent.setup>, password: string) {
  await user.type(screen.getByLabelText(/E-Mail/), "anna@werkstatt.de");
  await user.type(screen.getByLabelText(/^Passwort$/), password);
  await user.click(screen.getByRole("button", { name: "Anmelden" }));
}

beforeEach(() => {
  sessionStorage.clear();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  sessionStorage.clear();
});

describe("LoginPage", () => {
  it("logs in with valid credentials and shows the workshop area", async () => {
    stubFetch([
      { match: "/api/auth/login", body: { token: "test-token", employee: EMPLOYEE } },
      { match: "/healthz", body: { status: "ok" } },
    ]);
    const user = userEvent.setup();
    renderApp();

    await submitLogin(user, "geheim");

    await waitFor(() => {
      expect(screen.getByRole("heading", { level: 1, name: "Dashboard" })).toBeInTheDocument();
    });
  });

  it("shows one generic message on invalid credentials (401)", async () => {
    stubFetch([
      {
        match: "/api/auth/login",
        status: 401,
        body: { error: { code: "invalid_credentials", message: "Falsch." } },
      },
      { match: "/healthz", body: { status: "ok" } },
    ]);
    const user = userEvent.setup();
    renderApp();

    await submitLogin(user, "falsch");

    await waitFor(() => {
      expect(screen.getByText("E-Mail oder Passwort ist falsch.")).toBeInTheDocument();
    });
    expect(screen.getByRole("heading", { level: 1, name: "Werkstatt-Anmeldung" })).toBeInTheDocument();
  });

  it("shows the rate-limit warning on 429", async () => {
    stubFetch([
      {
        match: "/api/auth/login",
        status: 429,
        body: { error: { code: "too_many_requests", message: "Zu viele." } },
      },
      { match: "/healthz", body: { status: "ok" } },
    ]);
    const user = userEvent.setup();
    renderApp();

    await submitLogin(user, "geheim");

    await waitFor(() => {
      expect(
        screen.getByText("Zu viele Anmeldeversuche. Bitte warten Sie eine Minute."),
      ).toBeInTheDocument();
    });
  });

  it("starts neutral and only marks fields after a submit attempt", async () => {
    stubFetch([{ match: "/healthz", body: { status: "ok" } }]);
    const user = userEvent.setup();
    renderApp();

    expect(screen.queryByText("Bitte geben Sie eine gültige E-Mail-Adresse ein.")).not.toBeInTheDocument();
    expect(screen.queryByText("Bitte geben Sie Ihr Passwort ein.")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Anmelden" }));

    expect(await screen.findByText("Bitte geben Sie eine gültige E-Mail-Adresse ein.")).toBeInTheDocument();
    expect(screen.getByText("Bitte geben Sie Ihr Passwort ein.")).toBeInTheDocument();
  });
});

describe("workshop route protection", () => {
  it("redirects an unauthenticated workshop route to the login page", () => {
    stubFetch([{ match: "/healthz", body: { status: "ok" } }]);

    renderApp("/werkstatt");

    expect(
      screen.getByRole("heading", { level: 1, name: "Werkstatt-Anmeldung" }),
    ).toBeInTheDocument();
  });

  it("returns to the login page when a workshop request answers 401", async () => {
    sessionStorage.setItem(
      "werkstatt.auth",
      JSON.stringify({ token: "test-token", employee: EMPLOYEE }),
    );
    stubFetch([
      {
        match: "/api/orders",
        status: 401,
        body: { error: { code: "unauthorized", message: "Nicht angemeldet." } },
      },
      { match: "/api/auth/logout", status: 204, body: null },
    ]);

    function Probe() {
      useEffect(() => {
        void apiFetch("/api/orders").catch(() => undefined);
      }, []);
      return <p>Werkstattinhalt</p>;
    }

    render(
      <AuthProvider>
        <MemoryRouter initialEntries={["/werkstatt"]}>
          <Routes>
            <Route element={<RequireAuth />}>
              <Route path="/werkstatt" element={<Probe />} />
            </Route>
            <Route path="/werkstatt/anmelden" element={<LoginPage />} />
          </Routes>
        </MemoryRouter>
      </AuthProvider>,
    );

    expect(screen.getByText("Werkstattinhalt")).toBeInTheDocument();

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Werkstatt-Anmeldung" }),
      ).toBeInTheDocument();
    });
  });
});
