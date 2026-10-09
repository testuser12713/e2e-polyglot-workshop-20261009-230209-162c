"""Outbox for customer notifications.

The body of the notification is delivered by the ticket "Add the customer
notification to the worker's outbox"; this module already carries the final
signature the invoice flow calls with, so that ticket only has to fill it in.
"""

from __future__ import annotations

from typing import Any


def enqueue_customer_notification(conn: Any, order_number: str) -> None:
    """Record exactly one customer notification for the given order in the outbox.

    ``conn`` is a psycopg connection that already carries the surrounding
    transaction; ``order_number`` identifies the order the notification belongs
    to. Implemented by the outbox ticket, which owns the notification body.
    """
    raise NotImplementedError(
        "the customer outbox notification is implemented by the outbox ticket"
    )
