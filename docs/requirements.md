# Requirements baseline

Sources: owner's conversation and Mermaid schema attachment at
C:/Users/win/.codex/attachments/77652150-d9dc-4592-a38b-72deefcdca2b/Pasted text.txt.
UC_Description_Detailed_Exception_Integrated_v2.docx was fully read earlier in
this conversation; it is no longer available at its original Downloads path.
No current sequence document is supplied. docs/01-backend.md was fully reviewed
for Phase 4; confirm unresolved UC1 contracts before feature coding.
The final stack and clarifications from the owner override
the older schema's float money fields and public URL approach.

UC1 request and private drawings; UC2 estimate and quote; UC3 customer accepts
or rejects and terms are computed; UC4 deposit/company-issued PO and job opening;
UC5 required materials, stock check and reservation; UC6 supplier orders;
UC7 partial receipts, damage and stock/cost updates; UC8 production/material
usage/photos; UC9 delivery/invoice/payment, with payment after completion allowed.

UC8 five policies confirmed: any active Worker can start/log/complete any job;
Admin monitors/reads finance and enters actual labor cost, without production
actions. Start requires nonempty fully reserved current plan and does not consume
stock. Explicit incremental actual use consumes own reservation first, then free
stock, including extra/unplanned material; never use another job's reservation.
Completion releases unused reservations without adding physical stock; optional
private photos. Worker-entered whole labor minutes, issue-time material valuation,
Admin-entered labor cost, no assumed rate. See UC8.md. Both UC7 and UC8 still need
real PostgreSQL/S3/Docker validation; UC9 remains unapproved.

UC7 confirmed 2026-10-19: Admin/Worker receive and manage claims; financial details
and evidence stay Admin-only. Receive accepted orders partially, no excess. Damaged
units remain outstanding until good replacements; no refund/write-off. Admin enters
credit days per order, counted from full good receipt, with no default. One full
cash/bank settlement only after full receipt. Bank evidence required, cash receipt/
photo optional. Only good stock affects weighted average; no automatic reservation.
See UC7.md. Phase 10 is in progress, not yet integration-verified; UC8/UC9 unapproved.

UC6 confirmed 2026-10-18: Admin only manages suppliers/orders, responses and reads.
One supplier per order; multiple materials and flexible zero/many job references
(owner selected A). Allow consolidated orders exceeding shortages and general
replenishment. Do not add fixed allocations, incoming coverage or UC5 plan locks.
Placed quantities/prices/references are immutable; cancel/recreate unreceived
orders to change. Admin records supplier acceptance/rejection outside any Supplier
login. Rejection cancels the unreceived order with a mandatory reason/audit.
Round each exact line half-up to two decimals, then sum; THB/no VAT. No stock,
reservation or average-cost effect until UC7 receiving. See UC6.md. Migration
000009 preserves supplier/material/unit snapshots and immutable retry results.
No supplier receiving/payment implementation or new supplier notification channel
is authorized here; UC7-UC9 still require phase authorization.

Company threshold: <= 10,000 PO/no deposit; > 10,000 PO/50% deposit.
Individuals pay 50% first, no PO. Deposit/PO deadline is quotation acceptance
time plus 14 days, confirmed by owner. Pricing confirmed for Phase 5: minimum
(material+labor)*1.60, whole-request totals, two decimals, upward satang rounding.
Count rejection events rather than drafts; cancel on the sixth
rejection (more than five), confirmed by owner. Supplier usually settles in one payment.
Track delivery method self/third-party, not external logistics integration.

Frontend/backend alignment confirmed by owner: retain cancellation on the sixth
distinct customer quotation rejection, not the fifth. Admin Worker management
must require name, email and phone, in addition to the existing account fields
username/password/employee_no (position optional). Admin can add/edit,
deactivate/reactivate and change Worker passwords; no permanent deletion or
unconfirmed job reassignment. Worker reads/writes must support the required
contacts. Email/phone DTO, persistence, response and validation are implemented
with migration 000006. Owner confirmed Admin creates Workers with both contacts,
email syntax validation, international phone support and non-unique contacts.
Trim email, normalize phone formatting to optional + and 7-15 digits; no ownership
verification is claimed. Preserve legacy NULL contacts without fake backfills;
Admin fills them through PATCH. Reject attempts to clear populated contacts.
See docs/AUTH.md for the exact contract and migration/rollback boundaries.

Plan requirement quantities separately from quantities reserved and consumed.
Partial stock must not hide shortages. Stock and stock movements change in one
transaction with row locks; reconcile unconsumed reservations. Damaged stock
does not enter usable balance. MATERIAL_ORDER_ITEM needs partially_received.
Use NUMERIC for all money and exact quantities. Files are private S3 objects;
DB stores object keys/metadata, never bytes or permanent signed URLs.

Phase 4 approved on develop on 2026-10-14. Customer login is required, confirmed
by owner. Customers self-register; companies may have multiple customer users.
Company joining is invitation-only and company members share request access.
Uploads: PDF, DOC/DOCX, XLS/XLSX, TXT, JPEG/PNG, maximum 100,000,000 bytes per file.
Notification channels: LINE to one specific configured Admin user, not a group;
in-system notifications remain per-user. Credentials are private environment
configuration; real LINE sending is disabled by default.
The guide specifies documents/images up to 100 MB but no exact allowlist.
Use the owner's newer decisions above instead of the guide's stale fifth
rejection, undecided deadline and permanent URL wording.

Phase 4 implementation and isolated PostgreSQL/S3/Docker verification completed
2026-10-15. Pushed to the confirmed organization repository's develop branch;
main remains unchanged. API contracts and deployment limitations: docs/UC1.md.
UC2 was subsequently authorized on develop on 2026-10-15; UC3-UC9 still require
separate authorization. Phase 5 implementation and full PostgreSQL/S3/concurrency/
Docker verification passed on 2026-10-15 after a reversible runtime-socket Docker
recovery. Phase 5 implementation was pushed and verified on origin/develop at
d2b94ba; main remained 86d5be1. Completion acknowledgement is tracked in PLAN.md.

Phase 7 UC4 authorized 2026-10-18. Owner confirmed both active Admin/Worker may
approve/reject evidence; one deposit can be the rounded 50% minimum through total;
timely complete evidence may be reviewed later; missing required evidence expires
automatically; exactly one queued job opens after all required approvals. New
confirmation overrides original ER job cardinality. Migration 000007 stores
private attachment references, immutable submissions/review audit, actual approved
deposit/balance and deadline-cancellation audit. No public files, positive-payment
claims for zero totals, final invoice or new LINE event is inferred. See UC4.md,
UC4_VERIFICATION.md and PLAN.md. UC5-UC9 remain unapproved.

Phase 8 UC5 authorized 2026-10-18. Owner confirmed: Worker-editable material
requirements per queued job, partial reservations with visible shortages; Admin
manages units/materials and audited stock additions/corrections; queued reductions
release excess, increases reserve available stock, production locks edits; additions
recalculate weighted-average cost from quantity/unit cost, count corrections
retain existing cost. Corrections cannot undercut reservations. Quantities and
per-unit costs use NUMERIC(18,6); entered quantities never round silently.
Migration 000008 separates plans/reservations from actual consumption. Physical
stock is unchanged by reserving/releasing. No unit conversion or UC6-UC9 feature
is inferred. API/verification: UC5.md and UC5_VERIFICATION.md; completion in PLAN.md.
