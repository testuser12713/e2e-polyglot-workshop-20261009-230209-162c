"""Invoice calculation and persistence for the workshop invoice worker.

An order is turned into an invoice in two steps: the amount is computed from its
positions, then the invoice row is written to PostgreSQL. All money is handled in
whole cents so no floating point rounding can change a customer's bill.
"""

from __future__ import annotations

import logging
from collections.abc import Iterable
from dataclasses import dataclass
from decimal import ROUND_HALF_UP, Decimal

import psycopg
from config import Config, load_config
from outbox import enqueue_customer_notification

logger = logging.getLogger("worker.invoice")

VAT_PERCENT = 19
INVOICE_PREFIX = "RE-"
ORDER_PREFIX = "AU-"


@dataclass(frozen=True)
class Item:
    """One order position: either labor hours or a part with quantity and price."""

    kind: str
    description: str
    hours: Decimal | None
    quantity: int | None
    unit_price_cents: int | None


@dataclass(frozen=True)
class Order:
    """The order data the invoice is computed from."""

    id: int
    order_number: str
    items: tuple[Item, ...]


def _round_cents(value: Decimal) -> int:
    """Round a cent amount to a whole number, half away from zero."""
    return int(value.quantize(Decimal("1"), rounding=ROUND_HALF_UP))


def compute_net_cents(items: Iterable[Item], hourly_rate_cents: int) -> int:
    """Net amount in whole cents: hours times the workshop rate plus the parts."""
    total = 0
    for item in items:
        if item.kind == "labor":
            if item.hours is None:
                raise ValueError("labor item is missing hours")
            total += _round_cents(Decimal(item.hours) * hourly_rate_cents)
        elif item.kind == "part":
            if item.quantity is None or item.unit_price_cents is None:
                raise ValueError("part item is missing quantity or unit_price_cents")
            total += item.quantity * item.unit_price_cents
        else:
            raise ValueError(f"unknown item kind: {item.kind!r}")
    return total


def compute_invoice(items: Iterable[Item], hourly_rate_cents: int) -> tuple[int, int, int]:
    """Return ``(net_cents, vat_cents, gross_cents)`` for the given positions."""
    net_cents = compute_net_cents(items, hourly_rate_cents)
    vat_cents = (net_cents * VAT_PERCENT + 50) // 100
    gross_cents = net_cents + vat_cents
    return net_cents, vat_cents, gross_cents


def build_invoice_number(order_number: str) -> str:
    """Derive a stable invoice number from the unique order number."""
    suffix = (
        order_number[len(ORDER_PREFIX) :] if order_number.startswith(ORDER_PREFIX) else order_number
    )
    return f"{INVOICE_PREFIX}{suffix}"


def load_order(conn: psycopg.Connection, order_number: str) -> Order:
    """Load the order and its positions from PostgreSQL by order number."""
    row = conn.execute(
        "SELECT id, order_number FROM orders WHERE order_number = %s",
        (order_number,),
    ).fetchone()
    if row is None:
        raise LookupError(f"unknown order {order_number}")
    order_id, number = row
    item_rows = conn.execute(
        "SELECT kind, description, hours, quantity, unit_price_cents "
        "FROM order_items WHERE order_id = %s ORDER BY id",
        (order_id,),
    ).fetchall()
    items = tuple(
        Item(
            kind=kind,
            description=description,
            hours=hours,
            quantity=quantity,
            unit_price_cents=unit_price_cents,
        )
        for kind, description, hours, quantity, unit_price_cents in item_rows
    )
    return Order(id=order_id, order_number=number, items=items)


def insert_invoice(
    conn: psycopg.Connection,
    order_id: int,
    invoice_number: str,
    net_cents: int,
    vat_cents: int,
    gross_cents: int,
) -> None:
    """Insert the invoice row in the caller's transaction."""
    conn.execute(
        "INSERT INTO invoices "
        "(invoice_number, order_id, net_cents, vat_cents, gross_cents) "
        "VALUES (%s, %s, %s, %s, %s)",
        (invoice_number, order_id, net_cents, vat_cents, gross_cents),
    )


def process_order(order_number: str, config: Config | None = None) -> str:
    """Create the invoice for one order number and return its invoice number.

    Loads the order and its positions, computes net, 19 percent VAT and gross in
    whole cents, and inserts the invoice. Only the order number is ever logged.
    """
    if config is None:
        config = load_config()
    with psycopg.connect(config.database_url) as conn:
        order = load_order(conn, order_number)
        net_cents, vat_cents, gross_cents = compute_invoice(order.items, config.hourly_rate_cents)
        invoice_number = build_invoice_number(order.order_number)
        insert_invoice(
            conn,
            order.id,
            invoice_number,
            net_cents,
            vat_cents,
            gross_cents,
        )
        enqueue_customer_notification(conn, order.order_number)
    logger.info("invoice %s created for order %s", invoice_number, order_number)
    return invoice_number
