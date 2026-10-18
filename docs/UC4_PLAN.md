# Phase 7 — UC4 deposit/company PO and job opening

Authorized by owner on 2026-10-18. Branch: develop; no main push. Phase 7 is
started, not completed. No disputed financial/authorization rule is implemented.

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

## Business decisions needed before affected implementation

1. Who may verify/reject deposit and PO evidence: Worker, Admin, or both? Evidence
   submission is not confirmation that money reached the shop. Proposed: both
   active Admin and Worker, with reviewer/time/reason audit; owner must approve.
2. Does a required deposit have to equal the rounded 50%, or may one payment be
   any amount from that minimum through the total? Are several partial deposit
   payments supported in UC4, or only later invoice installments in UC9? Proposed
   first version: one payment >= required deposit and <= quoted total; no guessed
   cumulative partial-deposit behavior.
3. Does completed evidence submission before documents_due_at satisfy the
   deadline even if staff review happens later? Should missing evidence cause
   automatic cancellation after the deadline? Proposed: timely complete submission
   may be reviewed later; never cancel solely because staff review is pending.
   Rejected/late resubmission and exceptions must follow the owner's chosen policy.
4. Original ER says quotation-to-job is one-to-many; the UC4 flow describes opening
   a queued job without splitting. Proposed first version: exactly one job per
   approved quotation and idempotent automatic opening after all required evidence
   is verified. Confirm before adding a uniqueness constraint.

Proposals above are not confirmed requirements. Do not silently turn them into
defaults. Zero-total quotations already exist in UC2; resolve their evidence
handling when confirming the deposit policy rather than pretending money was paid.

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

The initial inspection found a clean develop at bdf3f75. A sandboxed remote
read failed inside Git's MSYS SSH runtime; retry with approved repository command
permissions before classifying any eventual push as blocked. Never force push,
change SSH credentials, rewrite history or claim a remote update without evidence.
