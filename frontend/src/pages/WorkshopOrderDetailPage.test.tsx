import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import WorkshopOrderDetailPage from "./WorkshopOrderDetailPage";

const ORDER_NUMBER = "AU-000001";

const BASE_HISTORY = [
  { status: "requested", changed_at: "2025-03-07T08:14:00Z" },
];

function initialOrder() {
  return {
    order_number: ORDER_NUMBER,
    status: "requested",
    desired_date: "2025-04-01",
    description: "Bremsen quietschen beim Anhalten.",
    customer: {
      id: 1,
      name: "Anna Muster",
      email: "anna@example.com",
      phone: "030 1234567",
    },
    vehicle: {
      id: 1,
      customer_id: 1,
      plate: "B-AB 1234",
      brand: "VW",
      model: "Golf",
      mileage_km: 84500,
    },
    items: [] as Array<Record<string, unknown>>,
    history: [...BASE_HISTORY],
    invoice: null,
  };
}

function jsonResponse(body: unknown, status = 200, statusText = "OK"): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

type FetchMock = ReturnType<typeof vi.fn>;

function parseBody(init: RequestInit | undefined): Record<string, unknown> {
  if (!init?.body || typeof init.body !== "string") {
    return {};
  }
  return JSON.parse(init.body) as Record<string, unknown>;
}

function renderDetail() {
  return render(
    <MemoryRouter initialEntries={[`/werkstatt/auftraege/${ORDER_NUMBER}`]}>
      <Routes>
        <Route path="/werkstatt/auftraege/:orderNumber" element={<WorkshopOrderDetailPage />} />
        <Route path="/werkstatt/auftraege" element={<p>Auftragsliste</p>} />
      </Routes>
    </MemoryRouter>,
  );
}

let fetchMock: FetchMock;
let statusResponse: () => Response;
let statusCalls: Array<{ path: string; body: Record<string, unknown> }>;
let itemResponse: () => Response;
let itemCalls: Array<{ path: string; body: Record<string, unknown> }>;

beforeEach(() => {
  let order = initialOrder();

  statusResponse = () =>
    jsonResponse({
      order_number: ORDER_NUMBER,
      status: "confirmed",
      history: [
        { status: "confirmed", changed_at: "2025-03-07T09:00:00Z" },
        ...BASE_HISTORY,
      ],
    });
  statusCalls = [];
  itemResponse = () =>
    jsonResponse({
      id: 9,
      kind: "part",
      description: "Bremsbelag",
      hours: null,
      quantity: 2,
      unit_price_cents: 8900,
    }, 201, "Created");
  itemCalls = [];

  fetchMock = vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
    const url = new URL(String(input));
    const method = (init?.method ?? "GET").toUpperCase();

    if (url.pathname === `/api/orders/${ORDER_NUMBER}` && method === "GET") {
      return jsonResponse(order);
    }
    if (url.pathname === `/api/orders/${ORDER_NUMBER}/status` && method === "POST") {
      const body = parseBody(init);
      statusCalls.push({ path: url.pathname, body });
      return statusResponse();
    }
    if (url.pathname === `/api/orders/${ORDER_NUMBER}/items` && method === "POST") {
      const body = parseBody(init);
      itemCalls.push({ path: url.pathname, body });
      return itemResponse();
    }
    return jsonResponse({ error: { code: "not_found", message: "Nicht gefunden." } }, 404, "Not Found");
  });

  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("WorkshopOrderDetailPage", () => {
  it("loads the order and shows customer, vehicle and positions", async () => {
    renderDetail();

    await waitFor(() => {
      expect(screen.getAllByText(ORDER_NUMBER).length).toBeGreaterThan(0);
    });
    expect(screen.getByText("Anna Muster")).toBeInTheDocument();
    expect(screen.getAllByText("B-AB 1234").length).toBeGreaterThan(0);
    expect(screen.getByText("Bremsen quietschen beim Anhalten.")).toBeInTheDocument();
    expect(screen.getByText("Noch keine Positionen erfasst.")).toBeInTheDocument();
    expect(screen.getAllByText("angefragt").length).toBeGreaterThan(0);
  });

  it("confirms the order and shows the new status without a reload", async () => {
    const user = userEvent.setup();
    renderDetail();

    await waitFor(() => {
      expect(screen.getAllByText(ORDER_NUMBER).length).toBeGreaterThan(0);
    });

    await user.click(screen.getByRole("button", { name: "Auftrag bestätigen" }));

    const dialog = screen.getByRole("dialog");
    expect(
      within(dialog).getByText(/Auftrag auf „Bestätigt" setzen\?/),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Bestätigen" }));

    await waitFor(() => {
      expect(screen.getAllByText("bestätigt").length).toBeGreaterThan(0);
    });
    expect(screen.getByText("Auftrag bestätigt.")).toBeInTheDocument();
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();

    expect(statusCalls).toHaveLength(1);
    expect(statusCalls[0].path).toBe(`/api/orders/${ORDER_NUMBER}/status`);
    expect(statusCalls[0].body).toEqual({ status: "confirmed" });
  });

  it("adds a part position and shows it in the list", async () => {
    const user = userEvent.setup();
    renderDetail();

    await waitFor(() => {
      expect(screen.getAllByText(ORDER_NUMBER).length).toBeGreaterThan(0);
    });

    await user.click(screen.getByRole("button", { name: "Teil" }));
    await user.type(screen.getByLabelText("Bezeichnung"), "Bremsbelag");
    await user.type(screen.getByLabelText("Menge"), "2");
    await user.type(screen.getByLabelText("Einzelpreis"), "89,00");
    await user.click(screen.getByRole("button", { name: "Position hinzufügen" }));

    await waitFor(() => {
      expect(screen.getByText("Bremsbelag")).toBeInTheDocument();
    });
    expect(screen.getByText("Position hinzugefügt.")).toBeInTheDocument();

    expect(itemCalls).toHaveLength(1);
    expect(itemCalls[0].path).toBe(`/api/orders/${ORDER_NUMBER}/items`);
    expect(itemCalls[0].body).toEqual({
      kind: "part",
      description: "Bremsbelag",
      quantity: 2,
      unit_price_cents: 8900,
    });

    expect((screen.getByLabelText("Bezeichnung") as HTMLInputElement).value).toBe("");
  });

  it("shows a rejected transition as an error and keeps the status", async () => {
    statusResponse = () =>
      jsonResponse(
        { error: { code: "invalid_transition", message: "Dieser Statuswechsel ist nicht erlaubt." } },
        409,
        "Conflict",
      );

    const user = userEvent.setup();
    renderDetail();

    await waitFor(() => {
      expect(screen.getAllByText(ORDER_NUMBER).length).toBeGreaterThan(0);
    });

    await user.click(screen.getByRole("button", { name: "Auftrag bestätigen" }));
    await user.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Bestätigen" }));

    await waitFor(() => {
      expect(screen.getByText("Dieser Statuswechsel ist nicht erlaubt.")).toBeInTheDocument();
    });
    const alert = screen.getByText("Dieser Statuswechsel ist nicht erlaubt.").closest(".alert");
    expect(alert).toHaveAttribute("data-error-code", "invalid_transition");

    expect(screen.getAllByText("angefragt").length).toBeGreaterThan(0);
    expect(screen.queryByText("bestätigt")).not.toBeInTheDocument();
  });

  it("keeps the position form neutral until a field was touched or submitted", async () => {
    renderDetail();

    await waitFor(() => {
      expect(screen.getAllByText(ORDER_NUMBER).length).toBeGreaterThan(0);
    });

    await userEvent.click(screen.getByRole("button", { name: "Teil" }));
    expect(screen.queryByText("Bitte geben Sie eine Bezeichnung ein.")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Position hinzufügen" }));
    expect(screen.getByText("Bitte geben Sie eine Bezeichnung ein.")).toBeInTheDocument();
    expect(itemCalls).toHaveLength(0);
  });
});
