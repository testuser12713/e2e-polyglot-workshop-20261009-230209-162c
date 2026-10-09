"""Outbox for customer notifications.

After the invoice for an order has been stored, the worker records exactly one
notification for the customer of that order in the Postausgang (outbox table).
The row carries the customer e-mail, the order number, the invoice number and a
German message text. The worker never sends a real e-mail - writing the row is
the whole side effect, so the notification can be delivered later by whoever
reads the outbox.
"""

from __future__ import annotations

import logging

import psycopg

logger = logging.getLogger("worker.outbox")

_CREATE_TABLE_SQL = """
CREATE TABLE IF NOT EXISTS outbox (
    id BIGSERIAL PRIMARY KEY,
    order_number TEXT NOT NULL UNIQUE,
    customer_email TEXT NOT NULL,
    invoice_number TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
)
"""

_SELECT_RECIPIENT_SQL = """
SELECT c.email, i.invoice_number
FROM orders o
JOIN vehicles v ON v.id = o.vehicle_id
JOIN customers c ON c.id = v.customer_id
JOIN invoices i ON i.order_id = o.id
WHERE o.order_number = %s
ORDER BY i.id DESC
LIMIT 1
"""

_INSERT_SQL = """
INSERT INTO outbox (order_number, customer_email, invoice_number, message)
VALUES (%s, %s, %s, %s)
ON CONFLICT (order_number) DO NOTHING
"""


def build_message(order_number: str, invoice_number: str) -> str:
    """Return the German notification text for one order.

    The text names the order and invoice numbers only; it never repeats the
    customer's name, e-mail or any other personal data.
    """
    return (
        f"Ihre Rechnung {invoice_number} zum Auftrag {order_number} wurde erstellt. "
        "Vielen Dank für Ihren Auftrag."
    )


def ensure_outbox_table(conn: psycopg.Connection) -> None:
    """Create the outbox table if it does not exist yet.

    The worker owns the outbox, so it makes sure its own table exists before the
    first notification is written - a freshly started product on an empty
    database must work without a manual migration.
    """
    conn.execute(_CREATE_TABLE_SQL)


def enqueue_customer_notification(conn: psycopg.Connection, order_number: str) -> None:
    """Record exactly one customer notification for the given order in the outbox.

    ``conn`` is a psycopg connection that already carries the surrounding
    transaction; ``order_number`` identifies the order the notification belongs
    to, and the customer e-mail plus the invoice number are read from the same
    database. The insert is idempotent per order, so re-processing a message
    still leaves exactly one notification. No e-mail is sent.
    """
    ensure_outbox_table(conn)
    row = conn.execute(_SELECT_RECIPIENT_SQL, (order_number,)).fetchone()
    if row is None:
        raise LookupError(f"no stored invoice for order {order_number}")
    customer_email, invoice_number = row
    conn.execute(
        _INSERT_SQL,
        (order_number, customer_email, invoice_number, build_message(order_number, invoice_number)),
    )
    logger.info("outbox notification stored for order %s", order_number)
