"""Tests for the customer notification outbox.

These run against the real PostgreSQL and Valkey the product declares. The schema
is created here so the suite works on a fresh, empty database, and only the rows
this module inserts are cleaned up afterwards. The outbox row is the only side
effect of processing a message: no e-mail is ever sent.
"""

from __future__ import annotations

import json
import logging
import os
import sys
import uuid
from pathlib import Path

import psycopg
import pytest
import valkey
from config import Config
from invoice import build_invoice_number, process_order
from main import QUEUE_NAME, consume_once
from outbox import build_message, enqueue_customer_notification

HOURLY_RATE_CENTS = 8900

CUSTOMER_NAME = "Erika Mustermann"
CUSTOMER_EMAIL = "erika.mustermann@example.test"
CUSTOMER_PHONE = "0151-9876543"

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


def _config() -> Config:
    return Config(
        database_url=os.environ["DATABASE_URL"],
        valkey_url=os.environ["VALKEY_URL"],
        hourly_rate_cents=HOURLY_RATE_CENTS,
    )


@pytest.fixture(scope="module")
def dsn() -> str:
    dsn = os.environ["DATABASE_URL"]
    with psycopg.connect(dsn) as conn:
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
            (CUSTOMER_NAME, CUSTOMER_EMAIL, CUSTOMER_PHONE),
        ).fetchone()[0]
        vehicle_id = conn.execute(
            "INSERT INTO vehicles (customer_id, plate, brand, model, mileage_km) "
            "VALUES (%s, %s, %s, %s, %s) RETURNING id",
            (customer_id, plate, "VW", "Golf", 100000),
        ).fetchone()[0]
        order_id = conn.execute(
            "INSERT INTO orders (order_number, vehicle_id, desired_date, description) "
            "VALUES (%s, %s, CURRENT_DATE, %s) RETURNING id",
            (order_number, vehicle_id, "Bremsen quietschen"),
        ).fetchone()[0]
        conn.execute(
            "INSERT INTO order_items (order_id, kind, description, hours) "
            "VALUES (%s, 'labor', %s, %s)",
            (order_id, "Bremsen erneuern", 1.5),
        )
        conn.commit()
    yield order_number, order_id, plate
    with psycopg.connect(dsn) as conn:
        conn.execute("DELETE FROM outbox WHERE order_number = %s", (order_number,))
        conn.execute("DELETE FROM invoices WHERE order_id = %s", (order_id,))
        conn.execute("DELETE FROM order_items WHERE order_id = %s", (order_id,))
        conn.execute("DELETE FROM orders WHERE id = %s", (order_id,))
        conn.execute("DELETE FROM vehicles WHERE id = %s", (vehicle_id,))
        conn.execute("DELETE FROM customers WHERE id = %s", (customer_id,))
        conn.commit()


def _outbox_rows(dsn: str, order_number: str) -> list[tuple]:
    with psycopg.connect(dsn) as conn:
        return conn.execute(
            "SELECT customer_email, invoice_number, message FROM outbox WHERE order_number = %s",
            (order_number,),
        ).fetchall()


def test_build_message_names_order_and_invoice() -> None:
    message = build_message("AU-1A2B3C", "RE-1A2B3C")
    assert "AU-1A2B3C" in message
    assert "RE-1A2B3C" in message
    assert message.endswith(".")


def test_process_order_stores_exactly_one_notification(dsn: str, seeded_order) -> None:
    order_number, _order_id, _plate = seeded_order
    invoice_number = process_order(order_number, _config())
    rows = _outbox_rows(dsn, order_number)
    assert len(rows) == 1
    customer_email, stored_invoice, message = rows[0]
    assert customer_email == CUSTOMER_EMAIL
    assert stored_invoice == invoice_number == build_invoice_number(order_number)
    assert order_number in message
    assert invoice_number in message


def test_reprocessing_same_order_keeps_one_notification(dsn: str, seeded_order) -> None:
    order_number, _order_id, _plate = seeded_order
    process_order(order_number, _config())
    # A retried queue message must not duplicate the notification.
    with psycopg.connect(dsn) as conn:
        enqueue_customer_notification(conn, order_number)
        conn.commit()
    rows = _outbox_rows(dsn, order_number)
    assert len(rows) == 1


def test_queue_message_stores_one_notification(dsn: str, seeded_order) -> None:
    order_number, _order_id, _plate = seeded_order
    client = valkey.Valkey.from_url(os.environ["VALKEY_URL"], decode_responses=True)
    try:
        client.delete(QUEUE_NAME)
        client.rpush(QUEUE_NAME, json.dumps({"order_number": order_number}))
        result = consume_once(client, _config())
    finally:
        client.close()
    assert result == build_invoice_number(order_number)
    rows = _outbox_rows(dsn, order_number)
    assert len(rows) == 1
    assert rows[0][0] == CUSTOMER_EMAIL


def test_logs_carry_no_personal_data(dsn: str, seeded_order, caplog) -> None:
    order_number, _order_id, plate = seeded_order
    with caplog.at_level(logging.INFO, logger="worker"):
        process_order(order_number, _config())
    logged = caplog.text
    for personal in (CUSTOMER_NAME, CUSTOMER_EMAIL, CUSTOMER_PHONE, plate):
        assert personal not in logged
    assert order_number in logged


def test_outbox_does_not_send_email() -> None:
    source = Path(sys.modules["outbox"].__file__).read_text(encoding="utf-8")
    # No mail transport is involved - the notification is only a database row.
    for forbidden in ("smtplib", "sendmail", "smtp"):
        assert forbidden not in source
