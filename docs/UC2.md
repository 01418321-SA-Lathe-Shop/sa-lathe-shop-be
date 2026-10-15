# UC2 — Estimation and quotations (Phase 5 in progress)

Implementation is on develop. Unit/build checks pass; real PostgreSQL,
concurrency, migration and Docker verification remain pending because the local
Docker Desktop engine cannot start. Do not treat the phase as complete yet.

## Confirmed pricing

- Costs and prices are **totals for the entire requested quantity**, not unit
  prices. Do not multiply the quoted total by request qty again.
- Minimum = `(material_cost + labor_cost) * 1.60`, rounded **up** to one satang.
- Two decimal THB amounts; **no VAT** or VAT fields.
- Amounts travel as JSON decimal **strings**, not floating-point numbers.
- Database uses NUMERIC(18,2); Go uses exact integer satang. Accepted range is
  0.00 through 9999999999999999.99. Reject excessive precision/exponents/negative
  amounts; reject calculations whose total/minimum exceeds the range.
- A final price may exceed the minimum, never fall below it. All three amounts
  are required for a saved draft. Zero costs yield a zero minimum; no additional
  unconfirmed fixed minimum fee is imposed.

Example: material 800.00 + labor 200.00 -> minimum 1600.00 for the whole request.
Material 0.01 + labor 0.01 -> minimum 0.04, not 0.03.

## Endpoints

All require the existing Bearer session. Only active **Workers** can estimate,
create/edit/send or refuse requests. Admin can read staff quotations/costs, but
does not automatically gain the Worker write role.

| Method | Path | Behavior |
|---|---|---|
| POST | `/quotations/estimate` | Preview material+labor and rounded minimum |
| POST | `/requests/{request_id}/quotations` | Create a draft/revision |
| PATCH | `/quotations/{quotation_id}` | Edit draft with expected lock version |
| POST | `/quotations/{quotation_id}/send` | Publish immutable price snapshot in-system |
| GET | `/quotations/{quotation_id}` | Authorized quotation view |
| GET | `/requests/{request_id}/quotations` | History; limit 1-100, offset >=0 |
| POST | `/requests/{request_id}/reject` | Worker refuses pending request with reason |
| GET | `/requests/{request_id}/rejection` | Scoped refusal reason/audit information |

Estimate body:

```json
{"material_cost":"800.00","labor_cost":"200.00"}
```

Create body:

```json
{"material_cost":"800.00","labor_cost":"200.00","final_price":"1600.00"}
```

Edit requires all three amounts plus `expected_lock_version` from the previous
response. Send body: `{"expected_lock_version":1}`. Editing/sending increments
the lock version; a stale value returns 409. Refusal body:
`{"reason":"Cannot machine this geometry"}` (1-2000 Unicode characters).
The refusal reason is customer-visible; do not put internal costing or secrets
in it. All JSON bodies reject unknown fields and multiple JSON values.

## Access and transitions

Staff can see drafts and internal costs. Customers see only non-draft quotations
for customer/company memberships they hold. Internal material/labor/total costs
and minimum price never appear in customer JSON, including history. Hidden or
missing quotation/request IDs both return 404. Missing/revoked sessions return
401; forbidden role 403; invalid inputs/minimum 400; stale/state conflicts 409.

Saving a first draft keeps request pending. Only one active draft/sent/approved
quotation can exist per request. Sending sets quotation sent, records sent_at and
updates request ongoing in the same transaction. Sent prices cannot be edited.
Revision version and edit lock_version are distinct. A new revision is permitted
only after the preceding quotation is rejected; it links to that predecessor,
increments commercial version and preserves history. UC3 will provide the actual
customer-decision endpoint in a later authorized phase; none exists now.

A Worker can refuse only a pending request; a sent/ongoing request cannot be
canceled by this UC2 action. Refusal reason/actor/time and canceled request state
commit together. Draft history is retained. This is not a customer rejecting a
quotation and does not count toward the later six-rejection rule.

All mutations recheck Worker activity/role inside the transaction. A request row
lock serializes draft creation, edits, sends and refusal. Optimistic edit tokens
prevent silent overwrites; DB constraints enforce minimum prices and uniqueness.

## Delivery boundary

Send means **publish for the customer in this system**. No quotation PDF, email,
LINE message or new notification event is claimed as delivered. Existing UC1
new-request alerts are unchanged. A future outbound quotation delivery contract
must explicitly identify channel/recipients; do not reuse new-request events.
The confirmed 14-day Deposit/PO clock starts at **customer acceptance**, not send.

## Run and migration

Apply migration 000004 before starting the updated API. It adds quotation,
quotation_cost and request_rejection; it does not mutate previous customer data.
Empty rollback is supported; rollback refuses once quotations/refusals exist.
Use backups and planned forward fixes for populated databases, never force a
dirty migration. No real/user database has been migrated by this implementation.

```powershell
./tasks.ps1 migrate-up
./tasks.ps1 run
go test ./internal/quotation
go build ./...
go vet ./...
go test ./...
```

Once Docker is healthy: `./scripts/verify-uc2.ps1`. It uses fresh random-named
Compose resources, isolated test DBs, random process-only credentials and an
empty env file, never existing databases or real LINE recipients. The new
PostgreSQL suite uses PHASE5_DATABASE_URL pointing specifically to
sa_lathe_shop_phase5_test. It also runs staff/UC1 regressions and Docker flow.
See UC2_VERIFICATION.md for current results and the startup blocker.
