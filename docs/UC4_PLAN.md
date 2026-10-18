# Phase 7 — UC4 deposit/company PO and job opening

Authorized by owner on 2026-10-18. Branch: develop; no main push. Phase 7 is
implemented and verified. The owner subsequently answered yes to all four policy
questions; the rules below are confirmed, not implementation assumptions.

## Reviewed sources

- AGENTS.md, PLAN.md, docs/requirements.md, docs/01-backend.md and docs/UC3.md.
- Original ER attachment: 77652150-d9dc-4592-a38b-72deefcdca2b/Pasted text.txt.
- UC3 exact-money acceptance snapshot and decision transaction, UC1 private file
  interfaces/content inspection/frozen objects, PostgreSQL evidence repository
  and API/worker composition roots.
- Original detailed Word/sequence files remain unavailable. Do not pretend the
  overview resolves their missing UC4 verification/error/exception contracts.

## Settled requirements

Use persisted quotation_acceptance_terms: individual requires a rounded-up 50%
deposit/no PO; company <=10000 requires PO/no deposit; company >10000 requires
both. documents_due_at is acceptance plus 336 hours, not a new submission clock.
Company issues its own PO. Customer accounts upload evidence; files must remain
private in S3, with only object keys/metadata in PostgreSQL and authorized signed
links. No VAT, public evidence links, DB file blobs or floating-point money.

## Owner-confirmed policies

1. Both active Admin and Worker may approve/reject deposit and PO evidence. Record
   reviewer, time and mandatory rejection reason. An upload is not verified money.
2. One deposit payment may range from the rounded 50% minimum to the quoted total.
   Do not accumulate several smaller payments in UC4. Record the actual approved
   deposit for the future invoice balance, without mutating UC3 minimum terms.
3. A complete set of evidence submitted within the 14-day acceptance deadline may
   be reviewed later. Automatically cancel if required evidence is missing.
   Rejected evidence is not a valid pending/approved set: replacement must be
   submitted before the same original deadline. No silent extensions or overrides.
4. Exactly one queued job per approved quotation/request opens atomically after
   all required evidence is approved. This newer explicit confirmation overrides
   the original ER's one-to-many quotation/job relationship.

Existing zero-total individual quotations retain their stored required-deposit
flag and accept a zero-amount evidence confirmation for staff review. This is
not a claim of a positive cash receipt or a new minimum quotation fee.

## Planned commit groups after confirmation

1. Confirmed rules, domain/evidence types, exact monetary validation and mock tests.
2. Evidence/PO/deposit/job/audit migration and repository transactions; lock the
   request/terms during eligibility and job opening. Define exact retry behavior.
3. Reuse validated private upload primitives without treating drawings as proof
   of payment; bind finalized evidence to its quotation, purpose and submitting
   actor. Keep technical file readiness separate from business verification.
4. Submission/review/status/history routes with strict JSON, current membership
   checks, role enforcement and evidence access isolation. No cost leaks.
5. Only confirmed deadline processing/notifications. Do not invent new LINE
   recipients/events; existing UC1 notifications remain unchanged by default.
6. Real PostgreSQL concurrent submit/review/open/expiry and rollback tests, S3
   content/freeze/privacy checks, migration up/down and disposable Docker flow,
   plus staff/Worker-contact/UC1-UC3 regression.
7. API/runbook/verification docs, PLAN completion, focused buildable commits and
   verified push to origin/develop. Stop before UC5 stock/reservation work.

The initial inspection found a clean develop at bdf3f75. The planning checkpoint
9d159e2 was pushed and verified on develop. Implementation proceeded in separate
domain, transactional repository and API/worker commits. Full expanded PostgreSQL,
S3, race/rollback and Docker verification passed in sa-lathe-uc4-92badc31; resources
were cleaned up. See UC4_VERIFICATION.md and PLAN.md for the completion push.
Never force push, rewrite history or push main without explicit authorization.
