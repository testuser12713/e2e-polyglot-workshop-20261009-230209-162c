import "@testing-library/jest-dom/vitest";
import { cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import WorkshopOrdersPage from "./WorkshopOrdersPage";

interface OrderRow {
  order_number: string;
  status: string;
  plate: string;
  customer_name: string;
  desired_date: string;
}

const ORDERS: OrderRow[] = [
  {
    order_number: "AU-000001",
    status: "in_progress",
    plate: "B-AB 1234",
    customer_name: "Anna Muster",
    desired_date: "2025-04-01",
  },
  {
    order_number: "AU-000002",
    status: "done",
    plate: "M-CD 5678",
    customer_name: "Bert Beispiel",
    desired_date: "2025-04-02",
  },
  {
    order_number: "AU-000003",
    status: "in_progress",
    plate: "B-EF 9999",
    customer_name: "Carla Probe",
    desired_date: "2025-04-03",
  },
];

function jsonResponse(body: unknown, status = 200, statusText = "OK"): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

function normalizePlate(value: string): string {
  return value.replace(/\s+/g, "").toUpperCase();
}

function filteredOrders(url: URL): OrderRow[] {
  const status = url.searchParams.get("status") ?? "";
  const plate = url.searchParams.get("plate") ?? "";
  return ORDERS.filter((order) => {
    if (status && order.status !== status) {
      return false;
    }
    if (plate && !normalizePlate(order.plate).includes(normalizePlate(plate))) {
      return false;
    }
    return true;
  });
}

function renderPage() {
  return render(
    <MemoryRouter initialEntries={["/werkstatt/auftraege"]}>
      <Routes>
        <Route path="/werkstatt/auftraege" element={<WorkshopOrdersPage />} />
        <Route
          path="/werkstatt/auftraege/:orderNumber"
          element={<p>Auftragsdetail</p>}
        />
      </Routes>
    </MemoryRouter>,
  );
}

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  fetchMock = vi.fn(async (input: RequestInfo | URL) => {
    const url = new URL(String(input));
    return jsonResponse(filteredOrders(url));
  });
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("WorkshopOrdersPage", () => {
  it("renders every order with a link to its detail page", async () => {
    renderPage();

    await waitFor(() => {
      expect(screen.getByText("AU-000001")).toBeInTheDocument();
    });
    expect(screen.getByText("AU-000002")).toBeInTheDocument();
    expect(screen.getByText("AU-000003")).toBeInTheDocument();

    expect(screen.getByRole("link", { name: "AU-000002" })).toHaveAttribute(
      "href",
      "/werkstatt/auftraege/AU-000002",
    );
    expect(screen.getAllByText("in Arbeit").length).toBeGreaterThan(0);
    expect(screen.getByText("fertig")).toBeInTheDocument();
  });

  it("requests the declared order list endpoint", async () => {
    renderPage();

    await waitFor(() => {
      expect(fetchMock).toHaveBeenCalled();
    });
    const requestedUrl = String(fetchMock.mock.calls[0][0]);
    expect(requestedUrl).toContain("/api/orders?");
    expect(requestedUrl).toContain("status=");
    expect(requestedUrl).toContain("plate=");
  });

  it("offers all five statuses plus 'Alle Status' in the filter", async () => {
    renderPage();

    const select = screen.getByLabelText("Status") as HTMLSelectElement;
    const labels = within(select)
      .getAllByRole("option")
      .map((option) => option.textContent);
    expect(labels).toEqual([
      "Alle Status",
      "Angefragt",
      "Bestätigt",
      "In Arbeit",
      "Fertig",
      "Abgeholt",
    ]);
  });

  it("keeps only matching rows after selecting the status 'In Arbeit'", async () => {
    const user = userEvent.setup();
    renderPage();

    await waitFor(() => {
      expect(screen.getByText("AU-000001")).toBeInTheDocument();
    });

    await user.selectOptions(screen.getByLabelText("Status"), "in_progress");

    await waitFor(() => {
      expect(screen.queryByText("AU-000002")).not.toBeInTheDocument();
    });
    expect(screen.getByText("AU-000001")).toBeInTheDocument();
    expect(screen.getByText("AU-000003")).toBeInTheDocument();

    const lastCall = fetchMock.mock.calls.at(-1);
    expect(String(lastCall?.[0])).toContain("status=in_progress");
  });

  it("narrows the list by license plate search", async () => {
    const user = userEvent.setup();
    renderPage();

    await waitFor(() => {
      expect(screen.getByText("AU-000001")).toBeInTheDocument();
    });

    await user.type(screen.getByLabelText("Kennzeichen suchen"), "m-cd");

    await waitFor(() => {
      expect(screen.queryByText("AU-000001")).not.toBeInTheDocument();
    });
    expect(screen.getByText("AU-000002")).toBeInTheDocument();
    expect(screen.queryByText("AU-000003")).not.toBeInTheDocument();

    const lastCall = fetchMock.mock.calls.at(-1);
    expect(String(lastCall?.[0])).toContain("plate=M-CD");
  });
});
