"""Entry point of the workshop invoice worker.

The worker blocks on the Valkey list ``invoices``. The API pushes one JSON
message per finished order, ``{"order_number": "AU-..."}``, and the worker hands
the order number to :func:`invoice.process_order`. Only the order number is ever
logged; customer data never reaches a log line.
"""

from __future__ import annotations

import json
import logging
import signal

import valkey
from config import Config, load_config
from invoice import process_order

logger = logging.getLogger("worker.main")

QUEUE_NAME = "invoices"


def decode_message(raw: str) -> str:
    """Extract the order number from one raw queue message."""
    payload = json.loads(raw)
    if not isinstance(payload, dict):
        raise ValueError("queue message is not a JSON object")
    order_number = payload.get("order_number")
    if not isinstance(order_number, str) or not order_number:
        raise ValueError("queue message carries no usable order_number")
    return order_number


def consume_once(client: valkey.Valkey, config: Config) -> str | None:
    """Pop a single message and process it; ``None`` when the queue was empty."""
    entry = client.blpop(QUEUE_NAME, timeout=1)
    if entry is None:
        return None
    _queue, raw = entry
    try:
        order_number = decode_message(raw)
    except (ValueError, KeyError, TypeError):
        logger.error("discarding malformed queue message")
        return None
    try:
        return process_order(order_number, config)
    except Exception:
        logger.exception("failed to process order %s", order_number)
        return None


def run(config: Config) -> None:
    """Block on the queue and process every order number it receives."""
    client = valkey.Valkey.from_url(config.valkey_url, decode_responses=True)
    logger.info("invoice worker listening on queue %s", QUEUE_NAME)
    while True:
        consume_once(client, config)


def main() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )
    config = load_config()
    signal.signal(signal.SIGTERM, _stop)
    run(config)


def _stop(_signum: int, _frame: object) -> None:
    logger.info("invoice worker stopping")
    raise SystemExit(0)


if __name__ == "__main__":
    main()
