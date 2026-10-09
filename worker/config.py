"""Environment configuration for the invoice worker.

Every value is read lazily inside :func:`load_config` and validated once when the
worker starts, so a misconfigured deployment fails with a message that names the
missing variable instead of a bare traceback at import time.
"""

from __future__ import annotations

import os
from dataclasses import dataclass


@dataclass(frozen=True)
class Config:
    """Everything the worker needs to run, read from the environment."""

    database_url: str
    valkey_url: str
    hourly_rate_cents: int


def _require(name: str) -> str:
    value = os.environ.get(name)
    if value is None or value.strip() == "":
        raise RuntimeError(
            f"{name} is not set. Declare it in RUN.json and export it before starting the worker."
        )
    return value


def load_config() -> Config:
    """Read and validate the worker configuration from the environment."""
    database_url = _require("DATABASE_URL")
    valkey_url = _require("VALKEY_URL")
    raw_rate = _require("HOURLY_RATE_CENTS")
    try:
        hourly_rate_cents = int(raw_rate)
    except ValueError as exc:
        raise RuntimeError(
            f"HOURLY_RATE_CENTS must be a whole number of cents, got {raw_rate!r}"
        ) from exc
    if hourly_rate_cents < 0:
        raise RuntimeError(f"HOURLY_RATE_CENTS must not be negative, got {hourly_rate_cents}")
    return Config(
        database_url=database_url,
        valkey_url=valkey_url,
        hourly_rate_cents=hourly_rate_cents,
    )
