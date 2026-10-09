import { useEffect, useState } from "react";
import { Link, NavLink, useLocation } from "react-router-dom";
import { useAuth } from "../lib/auth";

interface NavItem {
  to: string;
  label: string;
  end?: boolean;
}

const PUBLIC_LINKS: NavItem[] = [
  { to: "/termin", label: "Termin anfragen" },
  { to: "/mein-auftrag", label: "Status abrufen", end: true },
  { to: "/mein-auftrag#rechnung", label: "Rechnung ansehen" },
];

const WORKSHOP_LINKS: NavItem[] = [
  { to: "/werkstatt", label: "Dashboard", end: true },
  { to: "/werkstatt/auftraege", label: "Aufträge" },
];

export default function Navbar() {
  const location = useLocation();
  const { isAuthenticated, employee, logout } = useAuth();
  const [open, setOpen] = useState(false);

  useEffect(() => {
    setOpen(false);
  }, [location.pathname, location.hash]);

  const linkClass = ({ isActive }: { isActive: boolean }) =>
    `nav__link${isActive ? " nav__link--active" : ""}`;

  const renderPublicLinks = (onNavigate: () => void) =>
    PUBLIC_LINKS.map((item) => {
      const [path, hash = ""] = item.to.split("#");
      const isActive =
        location.pathname === path &&
        (hash ? location.hash === `#${hash}` : location.hash !== "#rechnung");
      return (
        <Link
          key={item.to}
          to={item.to}
          className={`nav__link${isActive ? " nav__link--active" : ""}`}
          onClick={onNavigate}
        >
          {item.label}
        </Link>
      );
    });

  const renderWorkshopLinks = (onNavigate: () => void) =>
    WORKSHOP_LINKS.map((item) => (
      <NavLink
        key={item.to}
        to={item.to}
        end={item.end}
        className={linkClass}
        onClick={onNavigate}
      >
        {item.label}
      </NavLink>
    ));

  return (
    <header className="nav">
      <div className="nav__inner container">
        <Link to="/" className="nav__brand">
          Kfz-Werkstatt
        </Link>

        <button
          type="button"
          className="nav__toggle"
          aria-expanded={open}
          aria-controls="primary-navigation"
          aria-label={open ? "Menü schließen" : "Menü öffnen"}
          onClick={() => setOpen((value) => !value)}
        >
          <span aria-hidden="true">{open ? "\u2715" : "\u2630"}</span>
        </button>

        <nav
          id="primary-navigation"
          className={`nav__menu${open ? " nav__menu--open" : ""}`}
          aria-label="Hauptnavigation"
        >
          <div className="nav__group">{renderPublicLinks(() => setOpen(false))}</div>

          <div className="nav__group nav__group--end">
            {isAuthenticated ? (
              <>
                {renderWorkshopLinks(() => setOpen(false))}
                {employee ? <span className="nav__user">{employee.email}</span> : null}
                <button
                  type="button"
                  className="btn btn--ghost btn--sm"
                  onClick={() => {
                    setOpen(false);
                    void logout();
                  }}
                >
                  Abmelden
                </button>
              </>
            ) : (
              <NavLink
                to="/werkstatt/anmelden"
                className={linkClass}
                onClick={() => setOpen(false)}
              >
                Werkstatt
              </NavLink>
            )}
          </div>
        </nav>
      </div>
    </header>
  );
}
