import { BrowserRouter, Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import RequireAuth from "./components/RequireAuth";
import { AuthProvider } from "./lib/auth";
import AppointmentPage from "./pages/AppointmentPage";
import HomePage from "./pages/HomePage";
import ImprintPage from "./pages/ImprintPage";
import LoginPage from "./pages/LoginPage";
import OrderLookupPage from "./pages/OrderLookupPage";
import PrivacyPage from "./pages/PrivacyPage";
import WorkshopOrderDetailPage from "./pages/WorkshopOrderDetailPage";
import WorkshopOrdersPage from "./pages/WorkshopOrdersPage";
import WorkshopPage from "./pages/WorkshopPage";

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<HomePage />} />
        <Route path="termin" element={<AppointmentPage />} />
        <Route path="mein-auftrag" element={<OrderLookupPage />} />
        <Route path="impressum" element={<ImprintPage />} />
        <Route path="datenschutz" element={<PrivacyPage />} />
        <Route path="werkstatt/anmelden" element={<LoginPage />} />
        <Route element={<RequireAuth />}>
          <Route path="werkstatt" element={<WorkshopPage />} />
          <Route path="werkstatt/auftraege" element={<WorkshopOrdersPage />} />
          <Route
            path="werkstatt/auftraege/:orderNumber"
            element={<WorkshopOrderDetailPage />}
          />
        </Route>
      </Route>
    </Routes>
  );
}

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <AppRoutes />
      </BrowserRouter>
    </AuthProvider>
  );
}
