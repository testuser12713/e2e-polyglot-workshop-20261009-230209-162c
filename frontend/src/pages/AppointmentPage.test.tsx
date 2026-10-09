import "@testing-library/jest-dom/vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import userEvent, { type UserEvent } from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import AppointmentPage from "./AppointmentPage";

function jsonResponse(body: unknown, status = 200, statusText = "OK"): Response {
  return {
    ok: status >= 200 && status < 300,
    status,
    statusText,
    text: async () => JSON.stringify(body),
  } as unknown as Response;
}

function futureIso(days = 7): string {
  const date = new Date();
  date.setDate(date.getDate() + days);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function renderPage() {
  return render(
    <MemoryRouter>
      <AppointmentPage />
    </MemoryRouter>,
  );
}

async function fillForm(user: UserEvent, desiredDate: string) {
  await user.type(screen.getByLabelText(/Name/), "Max Mustermann");
  await user.type(screen.getByLabelText(/E-Mail/), "max@example.com");
  await user.type(screen.getByLabelText(/Telefon/), "0170 1234567");
  await user.type(screen.getByLabelText(/Kennzeichen/), "B-AB 1234");
  await user.type(screen.getByLabelText(/Marke/), "VW");
  await user.type(screen.getByLabelText(/Modell/), "Golf");
  await user.type(screen.getByLabelText(/Kilometerstand/), "120000");
  fireEvent.change(screen.getByLabelText(/Wunschtermin/), {
    target: { value: desiredDate },
  });
  await user.type(screen.getByLabelText(/Problembeschreibung/), "Bremsen quietschen.");
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

describe("AppointmentPage", () => {
  it("submits the appointment and shows the assigned order number", async () => {
    const user = userEvent.setup();
    const desiredDate = futureIso();
    fetchMock.mockResolvedValue(
      jsonResponse({ order_number: "AU-AB12CD", status: "requested" }, 201, "Created"),
    );

    renderPage();
    await fillForm(user, desiredDate);
    await user.click(screen.getByRole("button", { name: "Termin anfragen" }));

    expect(await screen.findByText("AU-AB12CD")).toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledWith(
      expect.stringContaining("/api/orders"),
      expect.objectContaining({ method: "POST" }),
    );
    const body = JSON.parse(fetchMock.mock.calls[0][1].body as string);
    expect(body).toMatchObject({
      name: "Max Mustermann",
      email: "max@example.com",
      phone: "0170 1234567",
      plate: "B-AB 1234",
      brand: "VW",
      model: "Golf",
      mileage_km: 120000,
      desired_date: desiredDate,
      description: "Bremsen quietschen.",
    });
  });

  it("shows the API error message when the request fails", async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValue(
      jsonResponse(
        { error: { code: "plate_taken", message: "Dieses Kennzeichen ist bereits erfasst." } },
        409,
        "Conflict",
      ),
    );

    renderPage();
    await fillForm(user, futureIso());
    await user.click(screen.getByRole("button", { name: "Termin anfragen" }));

    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent("Dieses Kennzeichen ist bereits erfasst.");
    expect(screen.queryByText(/^AU-/)).not.toBeInTheDocument();
  });

  it("shows no field errors until the form is submitted or touched", async () => {
    const user = userEvent.setup();
    fetchMock.mockResolvedValue(jsonResponse({ status: "ok" }));

    renderPage();
    expect(screen.queryByText("Bitte geben Sie Ihren Namen ein.")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "Termin anfragen" }));

    expect(
      await screen.findByText("Bitte prüfen Sie die markierten Felder."),
    ).toBeInTheDocument();
    expect(screen.getByText("Bitte geben Sie Ihren Namen ein.")).toBeInTheDocument();
    expect(fetchMock).not.toHaveBeenCalled();
  });
});
