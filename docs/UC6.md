# UC6 — Supplier material orders

Apply migration **000009** before deploying this API. Handler -> Service ->
Repository remains unchanged; only the PostgreSQL repository runs SQL. No new
dependency, cache, queue, Supplier login or public file access was added.

## Confirmed behavior

Only an active **Admin** can access any procurement endpoint, including reads.
Workers still edit queued-job material plans under UC5. Supplier acceptance and
rejection are recorded by Admin after external contact, not by a Supplier account.

An order has one supplier and 1–100 distinct material lines, with 0–100 distinct
job references. These are request safety limits, not business quantity limits.
Job links are flexible references, not per-job allocations: orders can consolidate
jobs, exceed shortages or replenish general stock. No incoming-coverage counter,
shortage cap or UC5 plan lock is introduced. Any existing job can be referenced;
the reference does not authorize production, consumption or payment.

Quantities/unit costs are JSON decimal **strings**, NUMERIC(18,6). Quantities must
be positive; costs may be zero. Use material base units only, no conversions.
Each exact quantity * unit_cost rounds half-up to two decimals, then rounded
line amounts are summed as NUMERIC(18,2) THB. Example: 25 * 0.335 = **8.38**.
Two lines of 3 * 0.335 total **2.02**, not 2.01. Reject overflow, excess precision,
negative values and floats. No VAT. Zero-priced orders are permitted.

Placement, acceptance, rejection and cancellation never change stock, reservations
or average cost. Excess becomes general stock only at receipt in UC7, not now.
This supplier order is separate from UC4 customer-issued PO evidence.

## Endpoints

All require Authorization: Bearer with a current Admin session. Success is HTTP
200 (including exact replays); JSON responses use Cache-Control: no-store.

| Method | Path | Purpose |
|---|---|---|
| POST | /suppliers | Create active supplier |
| GET | /suppliers?search=steel&limit=20&offset=0 | Literal substring name search; includes inactive records |
| PUT | /suppliers/{supplier_id} | Replace contact/name/active fields with expected_version |
| POST | /material-orders | Place immutable order |
| GET | /material-orders?job_id={uuid}&limit=20&offset=0 | Latest orders, optionally by referenced job |
| GET | /material-orders/{order_id} | Current detail, snapshots, lines, links and ordered event history |
| POST | /material-orders/{order_id}/actions | Record accept/reject/cancel |

List limit is 1–100, default 20; offset non-negative. Unknown fields, invalid
IDs/decimals and invalid contacts return 400. Missing/invalid session: 401;
Worker/Customer: 403; missing record: 404; stale versions, reused retry keys,
inactive masters and invalid state transitions: 409. Existing error envelope:
`{"error":{"code":"...","message":"...","request_id":"..."}}`.

Supplier input:

```json
{"name":"Example Steel","email":"sales@example.com","phone":"+66812345678"}
```

Only name is required. Optional email must be a plain syntactically valid address,
not a display-name address. Optional phone is 7–15 digits with an optional leading
+; formatted punctuation is not accepted here. No contact uniqueness or ownership
verification is implied. PUT includes all three contact fields, is_active and
expected_version; omitted email/phone become empty. PUT is versioned, not a patch.
Supplier creation/updates record before/after audit in PostgreSQL; no deletion API.

Placement (generate a UUID key once and reuse it for retries):

```json
{
  "idempotency_key":"8b08c365-0696-42e1-b3ed-7daf52f8c906",
  "supplier_id":"a97da349-cbd6-48fc-8bba-e2692d42b0d5",
  "job_ids":["da0b5482-824c-4bc4-aa65-bb7c587e9bbd"],
  "expected_at":"2026-11-01T09:00:00+07:00",
  "lines":[{
    "material_id":"2f13e6bd-a07e-49c7-b8ea-c48a7c9f920b",
    "quantity":"25.000000",
    "unit_cost":"0.335000"
  }]
}
```

job_ids may be omitted/empty for general replenishment. expected_at is optional,
entered by Admin, normalized to UTC/microseconds and never assigned a guessed
lead time. API supports years 2000–9999; it does not enforce a delivery SLA.
New orders require active suppliers/materials. order_no is server-generated
MO-{UUID}; no gap-free/legal invoice numbering is claimed.

## State, audit and retry contract

| Current state | Command | Result |
|---|---|---|
| New | place | ordered / pending, version 1 |
| ordered / pending | accept | ordered / accepted, version +1 |
| ordered / pending | reject + reason | cancelled / rejected, version +1 |
| ordered / pending or accepted | cancel + reason | cancelled / unchanged response, version +1 |
| cancelled or response already recorded | another non-retry response | 409 |

```json
{
  "idempotency_key":"4b812864-0f3c-482b-bdc2-e10ac9ffce84",
  "expected_version":1,
  "action":"reject",
  "reason":"Supplier cannot fulfill this specification"
}
```

accept permits an optional reason. reject/cancel require a nonblank reason (max
2,000 characters). A recorded supplier response is final; cancel/recreate an
accepted commitment if it changes. There is no edit/delete/reopen endpoint.
Cancelled line quantities remain historical commitments, not active incoming stock.

Each placement/action records actor, action, reason, resulting version, timestamp,
canonical command and original result in immutable material_order_event. Same
UUID key + same actor + same payload returns that **original result**, even if
the order subsequently changed or its supplier was deactivated. Reordering job
IDs/lines and equivalent decimal spellings are canonicalized. Changed payload,
actor or target with the same key conflicts. A **new key intentionally creates a
new order**, even for identical quantities; there is no shortage-based deduplication.
Use GET after replay if current state is needed. Concurrent commands with the
same expected version have one winner. Stale/disabled Admins cannot replay/read.

Supplier contacts/version and material/unit names are frozen snapshots at placement.
Later master edits do not rewrite them. Prices, quantities, supplier and job links
cannot be changed; cancel/recreate before receipt instead.

## Persistence and concurrency

- supplier, supplier_audit, material_order, material_order_item,
  material_order_job and material_order_event; restrictive FKs retain history.
- Transactional live Admin authorization; advisory retry-key lock, then supplier
  shared lock and sorted material shared locks for stable snapshots. Job identity
  uses FKs; plans/reservations are not read or locked for shortage calculations.
- Action locks its order row; header change and audit/result commit together.
- PostgreSQL generates half-up item totals and checks the header sum and matching
  event at commit. Lines/links insert before the parent using deferred FKs;
  update/delete and append-after-placement are blocked by triggers.
- Paginated reads assemble detail in one SQL statement, not one pool connection
  per child. FK/job/created-time indexes support lists; search is literal substring,
  not a promised full-text search index. No performance benchmark is claimed.

Migration down refuses when procurement data exists. Back up/export and design an
explicit recovery migration instead of dropping real commitments/audit history.

## Not in this phase

UC7 must introduce partial/good/damaged receipts, receipt idempotency and ledger
integration, supplier settlement and receipt-aware cancellation under the same
order lock. Current UC6 cannot create a received order; cancellation-after-receipt
is not claimed as an implemented UC7 test. UC8 consumption and UC9 settlement
remain unapproved. No automatic supplier email/LINE message, PDF purchase-order
generation, shipping integration, unit conversion or new file endpoint is assumed.
