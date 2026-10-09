import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import OrderLookupPage from "./OrderLookupPage";

function jsonResponse(body: unknown, status = 200, statusText = "OK"): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

function mockFetch(handler: (url: string) => Response) {
  const fn = vi.fn(async (input: RequestInfo | URL) => handler(String(input)));
  vi.stubGlobal("fetch", fn);
  return fn;
}

const ORDER = {
  order_number: "AU-AB12CD",
  status: "in_progress",
  desired_date: "2026-03-07",
  description: "Bremsen prüfen und Beläge tauschen",
  plate: "B-AB1234",
  history: [
    { status: "requested", changed_at: new Date(2026, 2, 5, 8, 30).toISOString() },
    { status: "confirmed", changed_at: new Date(2026, 2, 6, 9, 0).toISOString() },
    { status: "in_progress", changed_at: new Date(2026, 2, 7, 9, 14).toISOString() },
  ],
};

const INVOICE = {
  invoice: {
    invoice_number: "AU-2026-000123",
    issued_at: new Date(2026, 2, 8, 10, 0).toISOString(),
    net_cents: 10000,
    vat_cents: 1900,
    gross_cents: 11900,
    items: [
      { description: "Arbeitszeit", quantity: 2.5, unit_price_cents: 8900 },
      { description: "Bremsbelag", quantity: 1, unit_price_cents: 4500 },
    ],
  },
};

async function fillAndSubmit(orderNumber = "AU-AB12CD", plate = "B-AB 1234") {
  const user = userEvent.setup();
  await user.type(screen.getByLabelText(/Auftragsnummer/), orderNumber);
  await user.type(screen.getByLabelText(/Kennzeichen/), plate);
  await user.click(screen.getByRole("button", { name: "Status abrufen" }));
  return user;
}

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

beforeEach(() => {
  window.location.hash = "";
});

describe("OrderLookupPage", () => {
  it("shows status, history and invoice for a matching order number and plate", async () => {
    const fetchMock = mockFetch((url) => {
      if (url.includes("/invoice")) {
        return jsonResponse(INVOICE);
      }
      if (url.includes("/api/public/orders/AU-AB12CD")) {
        return jsonResponse(ORDER);
      }
      return jsonResponse({ error: { code: "not_found", message: "not found" } }, 404, "Not Found");
    });

    render(<OrderLookupPage />);
    await fillAndSubmit();

    await waitFor(() => {
      expect(screen.getAllByText("in Arbeit").length).toBeGreaterThan(0);
    });

    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/public/orders/AU-AB12CD?plate=BAB1234"),
      expect.objectContaining({ method: "GET" }),
    );
    expect(screen.getByText("Bremsen prüfen und Beläge tauschen")).toBeInTheDocument();
    expect(screen.getByText("07.03.2026")).toBeInTheDocument();
    expect(screen.getByText("07.03.2026, 09:14 Uhr")).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText(/Rechnung Nr\. AU-2026-000123/)).toBeInTheDocument();
    });
    const invoice = document.getElementById("rechnung") as HTMLElement;
    expect(within(invoice).getByText("Arbeitszeit")).toBeInTheDocument();
    expect(within(invoice).getByText("2,5 h")).toBeInTheDocument();
    expect(within(invoice).getByText(/^100,00/)).toBeInTheDocument();
    expect(within(invoice).getByText(/^19,00/)).toBeInTheDocument();
    expect(within(invoice).getByText(/^119,00/)).toBeInTheDocument();
  });

  it("shows the API message when the combination does not match", async () => {
    mockFetch(() =>
      jsonResponse(
        {
          error: {
            code: "not_found",
            message:
              "Zu dieser Kombination aus Auftragsnummer und Kennzeichen wurde kein Auftrag gefunden.",
          },
        },
        404,
        "Not Found",
      ),
    );

    render(<OrderLookupPage />);
    await fillAndSubmit("AU-XXXXXX", "B-XX 9999");

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(
      "Zu dieser Kombination aus Auftragsnummer und Kennzeichen wurde kein Auftrag gefunden.",
    );
    expect(screen.queryByText(/Rechnung Nr\./)).not.toBeInTheDocument();
    expect(screen.queryByText("Verlauf")).not.toBeInTheDocument();
  });

  it("marks the invoice as not yet available when the API returns no invoice", async () => {
    mockFetch((url) => {
      if (url.includes("/invoice")) {
        return jsonResponse({ invoice: null });
      }
      return jsonResponse(ORDER);
    });

    render(<OrderLookupPage />);
    await fillAndSubmit();

    expect(
      await screen.findByText("Die Rechnung liegt noch nicht vor."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Sie erscheint hier, sobald der Auftrag fertiggestellt ist."),
    ).toBeInTheDocument();
  });
});
