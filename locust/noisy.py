"""Noisy-neighbour test.

A light "victim" tenant (default: Kestrel Fasteners) sends a small constant workload.
Optionally a heavy "neighbour" tenant (default: Ironval Castings) runs alonside it.
Compore the victim's latency with NEIGHBOUR_USERS=0 (baseline) and NEIGHTBOUR_USERS>0.

Environment:
    VICTIM_USERS (3), NEIGHBOUR_USERS (0), VICTIM_TENANT, NEIGHBOUR_TENANT,
    VICTIM_WINDOW_DAYS (1), NEIGHBOUR_WINDOW_DAYS (7)
Run with -u <VICTIM_USERS + NEIGHTBOUR_USERS> so exactly those users are spawned.
"""

import json
import os
import random
from datetime import datetime, timedelta, timezone

from locust import HttpUser, between, task

TOKENS_PATH = os.environ.get(
    "TOKENS_PATH",
    os.path.join(
        os.path.dirname(os.path.abspath(__file__)), "..", "loadtest", "tokens.json"
    ),
)

if not os.path.exists(TOKENS_PATH):
    raise SystemExit(
        f"{TOKENS_PATH} not found. Run the loader first (it writes tokens.json)"
    )

with open(TOKENS_PATH) as f:
    TOKENS = json.load(f)

VICTIM = os.environ.get("VICTIM_TENANT", "Kestrel Fasteners")
NEIGHBOUR = os.environ.get("NEIGHTBOUR_TENANT", "Ironvale Castings")
VICTIM_USERS = int(os.environ.get("VICTIM_USERS", "3"))
NEIGHBOUR_USERS = int(os.environ.get("NEIGHBOUR_USERS", "0"))
VICTIM_WINDOW_DAYS = int(os.environ.get("VICTIM_WINDOW_DAYS", "1"))
NEIGHBOUR_WINDOW_DAYS = int(os.environ.get("NEIGHBOUR_WINDOW_DAYS", "7"))

DATA_START = datetime(2026, 1, 1, tzinfo=timezone.utc)
DATA_DAYS = 270


def tokens_for(tenant_name: str) -> list[dict]:
    found = [t for t in TOKENS if t["tenant"] == tenant_name]
    if not found:
        raise SystemExit(f"No tokens for tenant {tenant_name!r} in {TOKENS_PATH}")
    return found


def rfc3339(dt: datetime) -> str:
    return dt.strftime("%Y-%m-%dT%H:%M:%SZ")


class _TenantUser(HttpUser):
    abstract = True
    tenant_name = ""
    label = ""
    window_days = 1

    def on_start(self):
        ident = random.choice(tokens_for(self.tenant_name))
        self.tenant_id = ident["tenant_id"]
        self.client.cookies.set("session", ident["token"])

    @task
    def defect_rate_by_product(self):
        start = DATA_START + timedelta(days=random.randint(0, DATA_DAYS - self.window_days))
        params = {"from": rfc3339(start), "to": rfc3339(start+timedelta(days=self.window_days))}
        with self.client.get(
            "/api/defect-rate/products",
            params=params,
            name=f"{self.label} [{self.tenant_name}]",
            catch_response=True,
        ) as resp:
            if resp.status_code != 200:
                resp.failure(f"HTTP {resp.status_code}")
                return
            try:
                rows = resp.json() or []
            except ValueError:
                resp.failure("invalid JSON")
                return

            foreign = {r["tenant_id"] for r in rows} - {self.tenant_id}
            if foreign:
                resp.failure(f"LEAK: user of {self.tenant_id} saw {sorted(foreign)}")
            else:
                resp.success()

class Victim(_TenantUser):
    fixed_count = VICTIM_USERS
    tenant_name = VICTIM
    label = "victim"
    window_days = VICTIM_WINDOW_DAYS
    wait_time = between(0.5, 1.0)


# Only defined when wanted, so a baseline run really has no neighbour
if NEIGHBOUR_USERS > 0:
    class Neighbour(_TenantUser):
        fixed_count = NEIGHBOUR_USERS
        tenant_name = NEIGHBOUR
        label = "neighbour"
        window_days = NEIGHBOUR_WINDOW_DAYS
        wait_time = between(0, 0.1)
