import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import WorkshopPage from "./WorkshopPage";

function jsonResponse(body: unknown, status = 200, statusText = "OK"): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

function tileValue(label: string): string {
  const tile = screen.getByText(label).closest(".stat-tile") as HTMLElement;
  return tile.querySelector(".stat-tile__value")?.textContent ?? "";
}

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchMock = vi.fn();
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("WorkshopPage", () => {
  it("shows open orders, orders finished today and the month revenue", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({ open_orders: 5, finished_today: 2, month_revenue_cents: 1248000 }),
    );

    render(<WorkshopPage />);

    await waitFor(() => {
      expect(screen.getByText("5")).toBeInTheDocument();
    });
    expect(screen.getByText("2")).toBeInTheDocument();

    expect(tileValue("Offene Aufträge")).toBe("5");
    expect(tileValue("Heute fertig")).toBe("2");
    expect(tileValue("Umsatz laufender Monat")).toBe("12.480,00\u00A0€");

    expect(String(fetchMock.mock.calls[0][0])).toContain("/api/reports/dashboard");
  });

  it("renders the revenue as 0,00 € for a month without invoices", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({ open_orders: 0, finished_today: 0, month_revenue_cents: 0 }),
    );

    render(<WorkshopPage />);

    await waitFor(() => {
      expect(tileValue("Umsatz laufender Monat")).toBe("0,00\u00A0€");
    });
    expect(tileValue("Offene Aufträge")).toBe("0");
    expect(tileValue("Heute fertig")).toBe("0");
  });

  it("marks the dashboard as busy while the report is loading", () => {
    fetchMock.mockReturnValue(new Promise<Response>(() => {}));

    render(<WorkshopPage />);

    const loadingText = screen.getByText("Wird geladen …");
    expect(loadingText).toBeInTheDocument();
    expect(loadingText.closest("[aria-busy='true']")).not.toBeNull();
  });

  it("shows the API error message through the shared ErrorBanner", async () => {
    fetchMock.mockResolvedValue(
      jsonResponse(
        { error: { code: "internal", message: "Das Dashboard ist nicht erreichbar." } },
        500,
        "Internal Server Error",
      ),
    );

    render(<WorkshopPage />);

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Das Dashboard ist nicht erreichbar.");
    expect(screen.queryByText("Offene Aufträge")).not.toBeInTheDocument();
  });
});
