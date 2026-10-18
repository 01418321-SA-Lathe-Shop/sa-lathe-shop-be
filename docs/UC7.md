# UC7 — Receiving, replacements and supplier settlement

Apply migration **000010** before deploying this API. Go/chi, PostgreSQL,
Handler -> Service -> Repository, private S3 and the existing worker are retained.
This records external receipts/payments; it does not execute bank transfers.

## Owner-confirmed permissions and workflow

- Admin and Worker receive goods and manage replacement claims. Supplier prices,
  costs, credit details, settlement and payment evidence are Admin-only.
- Suppliers have no accounts. Receiving requires an accepted supplier order.
  Partial deliveries are allowed; unexpected excess is rejected.
- Only good quantities enter usable stock and weighted-average cost. Damaged
  quantities never become usable stock and remain outstanding until replaced.
- Claims are replacement-only: no refund, credit note or write-off workflow.
  An order is fully received only when every ordered quantity is good, including
  good replacements. A shipment containing only damaged goods still blocks cancel.
- Admin enters credit days per order. Due time is full_received_at plus those
  many 24-hour days. There is no guessed/default credit period; 14-day customer
  Deposit/PO terms do not apply here. Zero days is supported for immediate terms.
- One full settlement, only after full good receipt: bank or cash. A bank transfer
  requires ready private evidence; cash evidence/photo is optional. No installments.

UC6 supplier management, placing/cancelling orders and full financial order reads
remain Admin-only. Workers use the separate limited receiving views below.
Worker material search omits avg_cost; Worker movement history omits unit/before/
after costs, private movement payload and free-form stock-adjustment reasons.
Admin retains these fields. Customer quotations/Worker estimation under UC2 are
unchanged; this restriction concerns supplier financial and inventory-cost data.

## API

Authorization: existing Bearer session; all responses Cache-Control: no-store.
Success/replay: 200. Invalid input/excess/invalid evidence: 400; no session: 401;
wrong role: 403; missing/inaccessible record: 404; stale version, reused retry key,
duplicate settlement or invalid state: 409. Existing JSON error envelope is retained.

| Method | Path | Access / behavior |
|---|---|---|
| GET | /receiving/orders?limit=20&offset=0 | Admin/Worker; accepted noncancelled orders, operational summaries |
| GET | /receiving/orders/{order_id} | Admin/Worker; quantities, deliveries, claims; finance only for Admin |
| POST | /receiving/orders/{order_id}/receipts | Admin/Worker; original or replacement shipment |
| POST | /receiving/orders/{order_id}/claims/{claim_id}/submit | Admin/Worker; record external supplier contact/claim |
| POST | /receiving/orders/{order_id}/credit-terms | Admin; enter credit days once |
| POST | /receiving/orders/{order_id}/settlement | Admin; one full-order settlement |
| POST | /material-orders/{order_id}/evidence/uploads | Admin; signed private upload intent |
| POST | /order-evidence/{evidence_id}/complete | Admin uploader; validate and freeze evidence |
| GET | /order-evidence/{evidence_id}/download | Admin; signed ready-file download |

List limit 1–100, default 20. A receipt has 1–100 distinct material/claim pairs.
These are API safety limits, not business quantity limits. Quantities are decimal
strings with at most six places, no float/exponent/negative/NaN. Always use the
placed item's base unit and immutable unit cost; clients cannot supply prices.

### Original receipt

```json
{
  "idempotency_key":"c19081e4-4312-44ef-8ab0-7e1b8e7ef686",
  "expected_version":3,
  "note":"Checked delivery against supplier order",
  "lines":[{
    "material_id":"ce180e70-c2e7-4a38-9862-c80e2b40424b",
    "good":"4.000000",
    "damaged":"2.000000"
  }]
}
```

Both good and damaged fields are required; their sum must be positive. On original
shipments, cumulative original good+damaged cannot exceed the ordered quantity.
Each damaged original line creates one pending_claim with its damaged quantity.
This is operational damage tracking, not a financial liability adjustment.

### Claim and replacement

Record contacting the supplier through phone/LINE/outside the API:

```json
{
  "idempotency_key":"427d32e8-d549-4a9d-bd4d-5bf6b2510238",
  "expected_version":4,
  "note":"Supplier agreed to send replacement units"
}
```

Claim status: pending_claim -> claimed -> resolved. Resolution is automatic only
when good replacements total the damaged quantity. No arbitrary resolve endpoint.
The claim note is required; later receipts preserve its actor/time/note.

A replacement receipt adds replacement_claim_id on the relevant line. It must
reference a claimed defect on the same order/material. Its good+damaged quantity
cannot exceed that claim's still-owed good quantity. Bad replacement attempts do
not fulfill the claim or create additional claims; the original claim stays open.
For example, 2 damaged original units + 1 bad replacement + 2 good replacements
has 3 historical damaged units, but exactly 2 owed/replaced units, not 3.

Original and replacement lines for a material may appear in one shipment as
distinct pairs. Receipts and claims never change the placed price/quantity.
Unknown materials/claims, cross-order claims, over-delivery and zero-only lines
are rejected atomically. Reactivate an inactive material before receiving good
stock, consistent with the existing active-catalog addition rule.

### Credit and settlement

Credit input is idempotency_key, expected_version and integer credit_days (0–36500).
Admin can set it once, before or after full receipt, but must set it before payment.
Setting it later never resets full_received_at or moves the due-date starting point.
Terms amendments require a separately approved workflow, not a silent edit.

```json
{
  "idempotency_key":"6a9cf26d-272e-4c15-b1e5-5073c2024fb7",
  "expected_version":8,
  "method":"bank",
  "paid_at":"2026-10-20T09:00:00+07:00",
  "evidence_id":"b10103ea-3543-43e8-b55d-3f691aab707a"
}
```

method is bank or cash. paid_at is required, not future-dated and not earlier than
the persisted full good receipt time. The amount is the immutable full order total;
there is no user-supplied amount/partial payment. Evidence must be ready and belong
to that order. Cash may omit evidence_id or attach a receipt/photo. Terms do not
prohibit early settlement after full receipt, nor payment after the due date.
Payment status is unpaid before full receipt/terms, pending when fully received
with terms and not settled, and paid after the single settlement.

## Private files

Start uploads with the existing file_name/content_type/byte_size shape. Reuse the
PDF, DOC/DOCX, XLS/XLSX, TXT, JPEG/PNG allowlist and 100,000,000-byte cap. Complete
validates content and freezes a separate final object using the source ETag. Ready
metadata cannot be edited. Failed completion requires a fresh upload intent.

Only object keys/metadata enter order_evidence; no bytes or permanent URLs.
Signed links expire after 10 minutes. Payment evidence is not request_attachment
and cannot be shared with customers through a linked job. Workers cannot start,
complete or download these files. Proof completion verifies file integrity, not
whether a bank transfer actually occurred; Admin records external settlement.

## Atomicity, audit and retries

material_receipt and its lines, material_claim, receipt-linked stock_movement,
order progress/full receipt time, settlement and material_order_event are protected
by transactions, restrictive FKs and append-only/consistency guards. Receipt/claim/
credit/payment each increment the shared order version and record actor/time,
canonical command and original result. Procurement cancellation and receiving use
the same order row lock, including all-damaged receipts.

Account authorization lock -> retry-key transaction lock -> order lock -> sorted
material locks. No job-plan locks are introduced. Each good line has exactly one
linked stock movement at the placed unit cost. Weighted average rounds half-up to
six places; reserved quantity is unaffected. Receipts cannot be appended/edited
after their transaction has finalized. Claimed/resolved quantities follow actual
good replacement rows, not editable counters.

Same UUID + actor + canonical input replays the original snapshot. Changed
input/actor/target conflicts. GET reads current state. A new key does not allow
over-receiving or a second settlement. Financial fields are absent from Worker
results, including retries. Credit/payment evidence and financial details stay
Admin-only even when Workers performed the receiving/claim steps.

The claim source references an original receipt line; replacement lines reference
that earlier claim. This deliberate FK cycle represents a time-ordered workflow:
original line -> claim -> later replacement, never a self-referential replacement.

## Boundaries and operation

Apply migrations with ./tasks.ps1 migrate-up before starting API/worker. Migration
down refuses if UC7 receipts, evidence, terms, settlement or new events exist.
Back up and design a recovery migration rather than deleting purchasing history.

No automatic reservation, supplier message, new LINE event, refund/write-off,
bank gateway, unit conversion, production consumption or UC8/UC9 implementation.
Workers explicitly recheck UC5 plans after stock arrives. No performance SLA is
claimed from functional tests. See UC7_VERIFICATION.md for checks actually run.
