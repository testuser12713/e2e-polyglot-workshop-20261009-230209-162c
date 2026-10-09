-- 0001_init.sql — initial schema for the workshop customer portal.
--
-- Applied idempotently on every API startup, so creating an object that already
-- exists is a no-op. PostgreSQL 18 provides gen_random_uuid() built in.

CREATE TABLE IF NOT EXISTS employees (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    name          text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    token_hash  text PRIMARY KEY,
    employee_id uuid NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    expires_at  timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS customers (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name       text NOT NULL,
    email      text NOT NULL,
    phone      text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS vehicles (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id uuid NOT NULL REFERENCES customers(id) ON DELETE CASCADE,
    plate       text NOT NULL UNIQUE,
    brand       text NOT NULL,
    model       text NOT NULL,
    mileage_km  integer NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS orders (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_number text NOT NULL UNIQUE,
    customer_id  uuid NOT NULL REFERENCES customers(id),
    vehicle_id   uuid NOT NULL REFERENCES vehicles(id),
    status       text NOT NULL DEFAULT 'requested'
                 CHECK (status IN ('requested', 'confirmed', 'in_progress', 'done', 'picked_up')),
    desired_date date NOT NULL,
    description  text NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS order_items (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id         uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    kind             text NOT NULL CHECK (kind IN ('labor', 'part')),
    description      text NOT NULL,
    hours            numeric(6,2),
    quantity         integer,
    unit_price_cents integer,
    created_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS order_history (
    id         bigserial PRIMARY KEY,
    order_id   uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    status     text NOT NULL,
    changed_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS invoices (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id       uuid NOT NULL UNIQUE REFERENCES orders(id) ON DELETE CASCADE,
    invoice_number text NOT NULL UNIQUE,
    issued_at      timestamptz NOT NULL DEFAULT now(),
    net_cents      integer NOT NULL,
    vat_cents      integer NOT NULL,
    gross_cents    integer NOT NULL
);

CREATE TABLE IF NOT EXISTS invoice_items (
    id               bigserial PRIMARY KEY,
    invoice_id       uuid NOT NULL REFERENCES invoices(id) ON DELETE CASCADE,
    description      text NOT NULL,
    quantity         numeric(10,2) NOT NULL,
    unit_price_cents integer NOT NULL
);

CREATE TABLE IF NOT EXISTS outbox (
    id              bigserial PRIMARY KEY,
    order_id        uuid NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    recipient_email text NOT NULL,
    subject         text NOT NULL,
    body            text NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_orders_status ON orders (status);
CREATE INDEX IF NOT EXISTS idx_orders_vehicle ON orders (vehicle_id);
CREATE INDEX IF NOT EXISTS idx_order_history_order ON order_history (order_id);
