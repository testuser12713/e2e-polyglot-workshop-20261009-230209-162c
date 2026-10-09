"""Tests for the invoice worker against a real PostgreSQL and Valkey.

The schema these tests use is created here so the suite runs on a fresh, empty
database. Only the rows this module inserts are cleaned up afterwards.
"""

from __future__ import annotations

import json
import os
import uuid
from decimal import Decimal

import psycopg
import pytest
import valkey
from config import Config
from invoice import Item, build_invoice_number, compute_invoice, process_order
from main import QUEUE_NAME, consume_once, decode_message

HOURLY_RATE_CENTS = 8900
LABOR_HOURS = Decimal("1.50")
PART_QUANTITY = 2
PART_UNIT_PRICE_CENTS = 1500
EXPECTED_NET_CENTS = 16350
EXPECTED_VAT_CENTS = 3107
EXPECTED_GROSS_CENTS = 19457

SCHEMA_SQL = """
CREATE TABLE IF NOT EXISTS customers (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT NOT NULL,
    phone TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS vehicles (
    id BIGSERIAL PRIMARY KEY,
    customer_id BIGINT NOT NULL REFERENCES customers(id),
    plate TEXT NOT NULL UNIQUE,
    brand TEXT NOT NULL,
    model TEXT NOT NULL,
    mileage_km INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    order_number TEXT NOT NULL UNIQUE,
    customer_id BIGINT NOT NULL REFERENCES customers(id),
    vehicle_id BIGINT NOT NULL REFERENCES vehicles(id),
    desired_date DATE NOT NULL,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'requested'
);
CREATE TABLE IF NOT EXISTS order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    kind TEXT NOT NULL,
    description TEXT NOT NULL,
    hours NUMERIC(8, 2),
    quantity INTEGER,
    unit_price_cents INTEGER
);
CREATE TABLE IF NOT EXISTS invoices (
    id BIGSERIAL PRIMARY KEY,
    invoice_number TEXT NOT NULL UNIQUE,
    order_id BIGINT NOT NULL REFERENCES orders(id),
    issued_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    net_cents BIGINT NOT NULL,
    vat_cents BIGINT NOT NULL,
    gross_cents BIGINT NOT NULL
);
"""


# The worker owns a dedicated PostgreSQL schema: the product schema the API
# migration creates in "public" has different tables (orders.customer_id is NOT
# NULL there), so the suite must never share it. Pinning the search_path keeps
# every connection - the test's and the worker's - on the worker's own tables.
WORKER_SCHEMA = "worker_test"


def _worker_dsn() -> str:
    """DATABASE_URL with search_path pinned to the worker's dedicated schema."""
    base = os.environ["DATABASE_URL"]
    sep = "&" if "?" in base else "?"
    return f"{base}{sep}options=-csearch_path%3D{WORKER_SCHEMA}"


def _config() -> Config:
    return Config(
        database_url=_worker_dsn(),
        valkey_url=os.environ["VALKEY_URL"],
        hourly_rate_cents=HOURLY_RATE_CENTS,
    )


@pytest.fixture(scope="module")
def dsn() -> str:
    dsn = _worker_dsn()
    with psycopg.connect(dsn) as conn:
        conn.execute(f"CREATE SCHEMA IF NOT EXISTS {WORKER_SCHEMA}")
        conn.execute(SCHEMA_SQL)
        conn.commit()
    return dsn


@pytest.fixture()
def seeded_order(dsn: str):
    order_number = "AU-" + uuid.uuid4().hex[:6].upper()
    plate = "B-" + uuid.uuid4().hex[:6].upper()
    with psycopg.connect(dsn) as conn:
        customer_id = conn.execute(
            "INSERT INTO customers (name, email, phone) VALUES (%s, %s, %s) RETURNING id",
            ("Test Kunde", "kunde@example.test", "0000"),
        ).fetchone()[0]
        vehicle_id = conn.execute(
            "INSERT INTO vehicles (customer_id, plate, brand, model, mileage_km) "
            "VALUES (%s, %s, %s, %s, %s) RETURNING id",
            (customer_id, plate, "VW", "Golf", 100000),
        ).fetchone()[0]
        order_id = conn.execute(
            "INSERT INTO orders (order_number, customer_id, vehicle_id, desired_date, description) "
            "VALUES (%s, %s, %s, CURRENT_DATE, %s) RETURNING id",
            (order_number, customer_id, vehicle_id, "Bremsen quietschen"),
        ).fetchone()[0]
        conn.execute(
            "INSERT INTO order_items (order_id, kind, description, hours) "
            "VALUES (%s, 'labor', %s, %s)",
            (order_id, "Bremsen erneuern", LABOR_HOURS),
        )
        conn.execute(
            "INSERT INTO order_items "
            "(order_id, kind, description, quantity, unit_price_cents) "
            "VALUES (%s, 'part', %s, %s, %s)",
            (order_id, "Bremsbelag", PART_QUANTITY, PART_UNIT_PRICE_CENTS),
        )
        conn.commit()
    yield order_number, order_id
    with psycopg.connect(dsn) as conn:
        conn.execute("DELETE FROM invoices WHERE order_id = %s", (order_id,))
        conn.execute("DELETE FROM order_items WHERE order_id = %s", (order_id,))
        conn.execute("DELETE FROM orders WHERE id = %s", (order_id,))
        conn.execute("DELETE FROM vehicles WHERE id = %s", (vehicle_id,))
        conn.execute("DELETE FROM customers WHERE id = %s", (customer_id,))
        conn.commit()


def test_compute_invoice_sums_labor_and_parts() -> None:
    items = [
        Item("labor", "Arbeit", LABOR_HOURS, None, None),
        Item("part", "Teil", None, PART_QUANTITY, PART_UNIT_PRICE_CENTS),
    ]
    net, vat, gross = compute_invoice(items, HOURLY_RATE_CENTS)
    assert (net, vat, gross) == (EXPECTED_NET_CENTS, EXPECTED_VAT_CENTS, EXPECTED_GROSS_CENTS)
    assert gross == net + vat


def test_compute_invoice_rounds_labor_to_whole_cents() -> None:
    items = [Item("labor", "Arbeit", Decimal("0.015"), None, None)]
    net, vat, gross = compute_invoice(items, 100)
    assert net == 2  # 1.5 cents rounds half up
    assert vat == 0
    assert gross == 2


def test_compute_invoice_rejects_unknown_kind() -> None:
    with pytest.raises(ValueError):
        compute_invoice([Item("mystery", "?", None, None, None)], HOURLY_RATE_CENTS)


def test_build_invoice_number_uses_order_number() -> None:
    assert build_invoice_number("AU-1A2B3C") == "RE-1A2B3C"


def test_process_order_creates_invoice_with_cents(dsn: str, seeded_order) -> None:
    order_number, order_id = seeded_order
    invoice_number = process_order(order_number, _config())
    assert invoice_number == build_invoice_number(order_number)
    with psycopg.connect(dsn) as conn:
        row = conn.execute(
            "SELECT net_cents, vat_cents, gross_cents FROM invoices WHERE order_id = %s",
            (order_id,),
        ).fetchone()
    assert row == (EXPECTED_NET_CENTS, EXPECTED_VAT_CENTS, EXPECTED_GROSS_CENTS)


def test_process_order_unknown_number_is_rejected(dsn: str) -> None:
    with pytest.raises(LookupError):
        process_order("AU-000000", _config())


def test_decode_message_reads_order_number() -> None:
    assert decode_message(json.dumps({"order_number": "AU-ABC123"})) == "AU-ABC123"


def test_decode_message_rejects_malformed_payload() -> None:
    with pytest.raises(ValueError):
        decode_message(json.dumps({"nope": 1}))


def test_queue_message_creates_invoice(dsn: str, seeded_order) -> None:
    order_number, order_id = seeded_order
    client = valkey.Valkey.from_url(os.environ["VALKEY_URL"], decode_responses=True)
    try:
        client.delete(QUEUE_NAME)
        client.rpush(QUEUE_NAME, json.dumps({"order_number": order_number}))
        result = consume_once(client, _config())
    finally:
        client.close()
    assert result == build_invoice_number(order_number)
    with psycopg.connect(dsn) as conn:
        row = conn.execute(
            "SELECT net_cents, vat_cents, gross_cents FROM invoices WHERE order_id = %s",
            (order_id,),
        ).fetchone()
    assert row == (EXPECTED_NET_CENTS, EXPECTED_VAT_CENTS, EXPECTED_GROSS_CENTS)
