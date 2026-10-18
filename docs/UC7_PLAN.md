# Phase 10 — UC7 receiving, defects and supplier settlement

Authorized 2026-10-19. Clean develop at ab0c224, UC6 complete. This is a planning
checkpoint, not a completed phase or implemented receipt/payment contract.
UC8-UC9 remain unapproved. Preserve Go/chi, PostgreSQL, private S3 and the existing
Handler -> Service -> Repository boundaries. Push develop only, never main.

## Sources reviewed

AGENTS.md, PLAN.md, docs/01-backend.md, docs/UC6.md, the original full Mermaid ER,
migrations 000008/000009, the stock-movement repository and private upload service.
The original detailed Word/sequence sources remain unavailable as previously
recorded. Do not infer missing policy from unrelated project files.

## Settled scope and invariants

- Record supplier deliveries with separate good and damaged quantities. Only good
  quantities enter usable physical stock and contribute to weighted-average cost.
- Good-stock balance, stock ledger, receipt/defect history and order progress must
  commit atomically. Reuse exact NUMERIC(18,6) quantities/per-unit costing and the
  six-place half-up weighted-average rule. Monetary settlement is NUMERIC(18,2),
  THB/no VAT. Never change placed order quantities, costs or historical snapshots.
- Flexible zero/many job references stay references: no per-job incoming allocation,
  shortage cap or automatic production. Preserve UC5's explicit Worker recheck/top-up
  rather than silently reallocating stock across jobs.
- Receipt, cancellation and payment commands need stable idempotency/version guards
  and order row locking. Prevent cancellation/rejection after any recorded receipt,
  including an all-damaged delivery, without bypassing UC6 audit constraints.
- Suppliers have no accounts. Payment records document external settlement; this
  phase does not authorize moving money through a bank/payment gateway.
- Evidence must be private S3 objects with DB keys/metadata and authorized signed
  links, not DB file bytes or public URLs. Current attachments are customer/request
  scoped; add an order-scoped boundary if UC7 evidence is required. Do not expose
  supplier files through linked customer jobs.

## Questions requiring owner confirmation before affected code

1. Permissions: keep receiving, claims, supplier settlement and reads Admin-only,
   or allow Workers to receive/manage claims? If Workers are allowed, separately
   identify whether supplier prices/payment evidence may be visible to them.
2. Delivery eligibility/limits: require recorded supplier acceptance first, allow
   partial deliveries and reject unexpected excess? Confirm replacement exceptions
   with the defect policy below; do not count damaged units as usable stock.
3. Defect resolution: replacement only, refund/credit, write-off, or all? Decide
   whether damaged quantities leave the order outstanding until replaced, or count
   as delivered with a separate claim. These choices determine completion, payable
   amounts and how later replacement deliveries are validated/costed.
4. Settlement model: cash and bank transfer, one full-order payment, unpaid until
   settled, with no credit terms/installments? The guide mentions CREDIT/pending,
   while the owner says suppliers usually settle once; neither defines actual
   methods, settlement adjustments or whether installments must be supported.
5. Payment timing: full receipt only, or before/during partial delivery as well?
   Clarify damaged-item amounts together with claim/refund rules. Do not assume
   paid means received or received means paid.
6. Evidence: require a completed private payment slip for bank transfer, while cash
   proof is optional? Preserve the existing document/image allowlist and 100 MB cap.

These are questions/proposals, not confirmed rules. No refund/write-off workflow,
credit period, changed access role or automatic supplier notification is authorized
by this document. Existing configured Admin LINE recipient/channel is unchanged;
do not invent new outbound UC7 events without a requirement.

## Planned implementation groups after confirmation

1. Confirmed domain/status rules, exact quantity/cost/settlement calculations and
   mock tests; no float arithmetic or speculative currency/unit conversion.
2. Receipt/line/defect/settlement/evidence migration and append-only audits; preserve
   UC6 placed items. Extend ledger application and order/event guards rather than
   disable their integrity protections. Guard destructive rollback of populated data.
3. Repository transactions with consistent account/order/sorted-material locking;
   same-key retry replay, concurrent receipt/payment prevention and atomic failure.
4. Authenticated receiving/history/approved claim/payment APIs, private order evidence
   and current progress reads. No UC8 consumption or new Supplier portal.
5. Test cumulative partial quantities, mixed/all-damaged shipments, approved claim
   settlement, receipt-versus-cancel races, duplicate commands, stock/cost rollbacks,
   payment races, permissions, private S3 and complete earlier-phase/Docker regression.
6. Update API/runbook/verification documentation and PLAN. Commit buildable logical
   groups as work progresses, push the completed phase to develop, verify remote and
   stop before UC8. No unmeasured performance claims or production-data migration.
