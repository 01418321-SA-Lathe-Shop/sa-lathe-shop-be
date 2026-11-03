# Use-Case Description Alignment (UC1–UC12)

Recorded 2026-11-03 against `Use_Case_Description_UC1-UC12.md`. Each UC document (`001`–`009`, `022`–`024`) has a section that lists what the description adds, what is implemented, and where the description conflicts with an owner-confirmed rule.

## 1. Precedence

1. Owner-confirmed decisions in `AGENTS.md`, `000-auth.md` and the UC documents.
2. The use-case description, where it adds steps or detail the confirmed rules do not cover.
3. The description, where it conflicts with a confirmed rule: the confirmed rule is kept and the conflict is listed below for an owner decision.

No code, migration or API contract was changed by this alignment. It only records where the description and the implementation differ.

## 2. Terminology

| Description | Project term |
| --- | --- |
| Technician (ช่าง) | Worker (domain role; the frontend may label it Staff) |
| Admin (ผู้ดูแลระบบ) | Admin |
| Super Admin | No such role. See `024-UC12.md` |
| Customer | Customer (individual or company) |
| Supplier | External party with no account |

## 3. Conflicts awaiting an owner decision

| # | Topic | Description says | Confirmed rule kept | UC doc |
| --- | --- | --- | --- | --- |
| 1 | Procurement actor | Technician places orders (UC6) | Admin only | 006 |
| 2 | Customer rejections | 5th rejection cancels automatically | Rejections 1–5 allow revision; 6th cancels | 003 |
| 3 | Deposit amount | `ROUND(50%, 2)`, exact match | Round a 50% deposit up to a satang; any amount from the minimum through 100% | 004 |
| 4 | Deposit verification | System compares amount with the due amount | Admin or Worker reviews each submission | 004 |
| 5 | Supplier payment | Recorded when goods are received (CREDIT → pending) | Admin sets credit days per order; one full settlement after full good receipt | 007 |
| 6 | Defective goods | May end as WRITTEN_OFF | Replacement only; no refund or write-off | 007 |
| 7 | Actual labor cost | Technician enters actual cost | Admin enters actual labor cost; Worker logs time | 008 |
| 8 | Completion photo | Required | Optional | 008 |
| 9 | Delivery method | `pickup` or delivery with `shipping_cost` | `self` or `third_party`, no shipping cost | 009 |
| 10 | Customer payment | Technician records the payment | Customer submits bank evidence; Admin reviews; Admin records cash | 009 |
| 11 | Worker duplicates | Phone or email duplicates blocked | Contacts are non-unique | 001, 023 |
| 12 | Existing person | Adopt existing person as Worker or Admin | Never adopt unrelated accounts | 023, 024 |
| 13 | Admin levels | Super Admin creates Admins | Only the bootstrap command creates the first Admin; no super-admin tier | 024 |
| 14 | Sign-up address | Structured fields and 5-digit postal code | One free-text address (migration 000018) | 001 |

## 4. Gaps the description requires

These are described in the source but not implemented. Each needs an owner decision on whether to build them.

- Estimated cost and estimated time on a job (Q2.2, Q4.3). Not in the schema or API. The ER comparison in `021-er-schema-comparison.md` lists the same omission.
- Worker refusal of an ongoing request (Q2.E1 allows `ongoing`; the implementation refuses `pending` only).
- Send a request back for more drawing details, keeping it pending (UC2 Alt-Step 5). No endpoint.
- Alternative supplier search after a supplier rejects an order (Q6.E2). No endpoint.
- Notify jobs waiting for a received material to start (Q7.7). No notification. Open item D13 in the frontend decision register.
- Rename a unit or edit a conversion factor in place (Q10.5, Q10.6). See `022-UC10.md`.
- Hire date validation for Workers (Q11, Step 3.3). Not confirmed.
- Super Admin creation (UC12). See `024-UC12.md`.

## 5. Implementation differences that are not conflicts

- UC5: the description reserves the full requirement even when short. The implementation allocates what is available and exposes the remaining shortage, which the confirmed partial-allocation rule requires.
- UC9: invoice status is derived from payment allocations rather than stored as a column. The confirmed rule (`unpaid` → `partial` → `paid` as amounts are allocated) is kept.
- UC9: the invoice due date is `issued_at + credit_days × 24 hours`, with credit days set explicitly by Admin. The description gives a due date without a rule.

## 6. Documents changed by this alignment

- `001-UC1.md` to `009-UC9.md`: alignment sections added.
- `022-UC10.md`, `023-UC11.md`, `024-UC12.md`: new.
- `025-uc-spec-alignment.md`: this document.
- Frontend repository: `docs/014-use-case-index.md`, `docs/024-USECASE10-12.md` and per-UC pages `015`–`023`.
