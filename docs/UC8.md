# UC8 — Production, actual use, time and completion

Implemented and PostgreSQL/S3/Docker verified on 2026-10-19, including the UC7
verification dependency. Apply migrations through 000011 using the normal
migration workflow before running this API; no existing database was migrated
by verification. See UC8_VERIFICATION.md. UC9 delivery, invoicing and returned-work reopening are not
implemented here. The existing Go/chi layered architecture is retained.

## Confirmed rules

Any active Worker may start/log/complete any job; no assignment restriction.
Admin monitors and sees costs, but cannot start, issue, log time, complete or
upload production photos. The explicit exception is Admin actual labor-cost entry.
Customers/Suppliers cannot inspect production data/photos. Worker results never
contain supplier prices, material valuation or actual labor/total cost.

Start requires queued status, the current material-plan version, and a nonempty
fully reserved plan. Starting only changes status/timestamp; it never consumes
the plan. Existing plan edits/rechecks remain queued-only once production starts.

Each actual usage entry is incremental, not a replacement cumulative total.
Consume this job's reservation first, then genuinely free stock. Extra quantity
or previously unplanned material is allowed, but cannot use another job's
reservation. Inactive material may consume its already-owned reservation; extra
or unplanned inactive stock requires Admin reactivation first. Physical stock
decreases only when actual use is recorded; average cost is retained on issues.

Completion releases every unused reservation and marks the job completed. These
units were never consumed, so release does NOT add physical stock. Photos are
optional; no automatic full-plan usage or guessed time/cost is recorded. A job
may complete with no usage/time entered; that does not imply all planned material
was used or that elapsed wall time equals actual labor. No mandatory gate was
specified for those optional records.

Workers enter positive whole labor minutes explicitly. Material cost uses the
weighted-average per-unit snapshot at each issue; each exact quantity * unit cost
rounds half-up to two THB decimals, and those immutable issue amounts are summed.
Quantity/unit cost remain NUMERIC(18,6); money NUMERIC(18,2), no float/VAT.
Admin enters actual labor cost, including an explicit zero if appropriate; no
hourly rate is inferred. Labor cost is unknown/null until entered, and total cost
is null until labor cost is known. Admin may revise the labor total with a new
versioned command, preserving prior command/result snapshots in the audit.

## API

Existing Bearer authentication, no-store responses, strict JSON and normal error
envelope. Successful writes and exact retries return 200. 400 invalid input,
precision/reference/overflow; 401 no current session; 403 wrong role; 404 missing
record; 409 stale version, not-ready/closed state, retry conflict or unavailable
free stock. Private SQL errors/financial data are not exposed.

| Method | Path | Access |
|---|---|---|
| GET | /production/jobs?limit=20&offset=0 | Admin/Worker; queued/in-progress/completed summaries |
| GET | /production/jobs/{job_id} | Admin/Worker; operations/history/photos; finance only Admin |
| POST | /production/jobs/{job_id}/start | Worker |
| POST | /production/jobs/{job_id}/usage | Worker |
| POST | /production/jobs/{job_id}/time | Worker |
| POST | /production/jobs/{job_id}/complete | Worker |
| PUT | /production/jobs/{job_id}/labor-cost | Admin |
| POST | /production/jobs/{job_id}/photos/uploads | Worker; in-progress job |
| POST | /job-photos/{photo_id}/complete | Active Worker uploader; still in-progress |
| GET | /job-photos/{photo_id}/download | Admin/Worker; ready private photo |

Lists use limit 1–100, default 20; offset nonnegative. A usage command has 1–100
distinct materials. Minutes per entry: 1–1,000,000. These are API safety limits,
not business order-quantity limits. Quantities are decimal strings, positive and
at most six places; no JSON number/exponent/negative/NaN or excess precision.

Start body:

```json
{
  "idempotency_key":"dbe8d729-7dd0-4374-af2c-8ed24b9a1806",
  "expected_version":0,
  "expected_plan_version":1
}
```

expected_version is the **production version**, initially 0; it is not the stock
or material-plan version. GET production detail returns both version fields.
expected_plan_version must match the current nonempty fully reserved plan.

Usage body:

```json
{
  "idempotency_key":"57582d89-ae96-4278-b068-17835858eed1",
  "expected_version":1,
  "note":"Actual steel issued to mould production",
  "lines":[{"material_id":"0f4a893a-220e-4497-8f2a-61c55f10c1b3","quantity":"2.500000"}]
}
```

The server chooses valuation, reservation consumption, actor and timestamps.
Caller-supplied prices/costs/stock balances/status are rejected as unknown fields.
Usage snapshots retain material/unit names and unit cost even if masters or
average costs change later. Unplanned materials do not silently edit the frozen
job plan or create a reservation.

Time uses idempotency_key, expected_version, minutes and optional note (max 2,000
characters). It appends actual minutes, not a stopwatch/start-end duration. Complete
uses only idempotency_key and expected_version. After completion, new usage/time
and start/complete commands conflict. Admin labor-cost PUT uses the same command
fields plus decimal-string amount, and is permitted in-progress or completed.
There is no destructive edit/delete for usage/time/events or manual reopening.

## Private photos

Upload body uses file_name, content_type, byte_size. JPEG/PNG only, maximum
100,000,000 bytes; validate extension, MIME and image content before freezing.
Optional photos are independent of required customer PO/deposit or supplier proof.
Only object keys/metadata enter job_photo. Signed links expire after 10 minutes;
no file bytes, public URLs or reusable payment/request attachment binding.

Upload/complete are permitted only while in-progress, and completion rechecks job
status and uploader identity after S3 inspection. If the job completed meanwhile,
the intent fails rather than silently attaching a new photo to closed production.
Failed completion requires a new intent, subject to the same status/permissions.
Ready photos remain staff-readable after completion. Admin/customer cannot upload
and customer cannot download. Photo metadata changes do not increment production
version or falsely record a stock/labor action.

## Transactions and persistence

Migration 11 adds production job version/timestamps/labor cost, production_event,
job_material_usage, job_time_entry and job_photo. It extends stock_movement with
an immutable usage link and material_reservation with qty_consumed. Reservation
status is reserved while any quantity remains; consumed if exhausted with actual
reservation use, otherwise released. The production detail also shows reserved,
reservation_consumed, actual used and released quantities separately.

Lock order: account authorization -> retry key -> request -> job -> sorted
material UUIDs. Pure production domain policy is evaluated again under these
locks. Usage, own-reservation decrement, cached reserved aggregate, physical stock,
ledger, valuation and job event commit atomically. Completion releases remaining
reservations while still in-progress, then marks completed in the same transaction.
It never changes request completion/invoice/payment status.

Database guards validate the ledger snapshot and linked usage, consumed totals,
nonempty fully reserved start, versioned status/timestamps, no leftover completion
reservations and matching production event. Bare balance/history edits are rejected.
Exact aggregate monetary overflow rolls back the whole command.

Same key + actor + canonical job/action/input replays the original role-safe
snapshot even after completion; changed input/actor/job conflicts. Material order
and decimal spellings canonicalize. GET returns current state. A new key denotes
another intentional incremental issue/time entry, not a retry. Never resend with
a new key after an uncertain response without checking the original command.

Migration down refuses if production history/photos/versioned job data exists.
Use backups and a planned forward recovery, not deletion of consumption history.

## Not included

No assignment system, payroll/hourly rate, stopwatch, negative usage correction,
inventory return, pause/rework/cancel-after-consumption, scheduling, new notification
channel, customer photo sharing, delivery/invoicing or UC9 returned-work reopening.
No throughput/latency or end-to-end readiness claim before real verification.
