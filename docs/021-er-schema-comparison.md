# ER Diagram vs Implemented Database Schema

Source: the original ER diagram supplied by the project owner, compared against migrations `000001` through `000018`. This is a reference mapping for reviewers. It does not change any migration or UC contract; the migrations and the UC guides remain authoritative.

The implemented database covers the same business areas as the ER, but it is a more detailed design. Many entities are split, renamed, or replaced, and it adds audit, idempotency, and private-file tables that the ER does not have.

## Closely matching entities

| ER entity | Implemented table(s) | Notes |
| --- | --- | --- |
| `JOB_REQUEST` | `job_request` | Same statuses (pending, ongoing, completed, canceled), `deadline`, `qty`, `source`. |
| `REQUEST_ATTACHMENT` | `request_attachment` | Stores a private object key, never a URL. |
| `QUOTATION` | `quotation` | Versioned per request; see revisions below. |
| `QUOTATION_COST` | `quotation_cost` | `NUMERIC(18,2)` instead of `double`. |
| `JOB` | `job` | Statuses queued, in_progress, completed, cancelled. |
| `JOB_PHOTO` | `job_photo` | Private object storage. |
| `MATERIAL` | `material` | `qty_on_hand` and `avg_cost` as in the ER, plus `qty_reserved`, `version`, `is_active`, `created_by`. |
| `UNIT` | `unit` | |
| `UNIT_CONVERSION` | `unit_conversion` | |
| `SUPPLIER` | `supplier` | Plus `is_active`, `version`, and `supplier_audit`. |
| `DELIVERY` | `delivery` | Method `self` or `third_party`. |
| `INVOICE` | `invoice` | One invoice per job. |
| `PAYMENT` | `customer_payment` | Bank or cash method, with approval status and evidence. |
| `PAYMENT_ALLOCATION` | `payment_allocation` | Each payment allocates to one invoice. |
| `MATERIAL_ORDER` | `material_order` | Partly matching; see differences. |
| `MATERIAL_ORDER_ITEM` | `material_order_item` | Partly matching; see differences. |
| `WORKER` | `worker` | `employee_no`, `position`, `hired_at`. |
| `ADMIN` | `admin` | Linked to `account`. |
| `MATERIAL_DEFECT_LOG` | `material_claim`, `material_receipt_line` | Replacement-only claims. |

## Differences by area

### Identity and customers

- The ER uses `PERSON` as a supertype with `CUSTOMER`, `WORKER` and `ADMIN` subtypes. The implementation uses `account` for login, with roles admin, worker and customer. `worker` and `admin` attach to `account`.
- `customer` is a company or individual record, not a person. Accounts reach it through `customer_member` (with an owner flag) and one-use `customer_invitation`.
- `ADDRESS` (line 1, line 2, district, province, postal code, default flag) becomes `customer_address` with one `address` text field.
- `tax_id` and billing contact fields were added in migration 000017.

### Quotations and acceptance

- The ER status `requote` does not exist. Revisions are a chain through `previous_quotation_id`, with a `version` per request.
- `DEPOSIT` and `PURCHASE_ORDER` become `term_evidence` (kind `deposit` or `po`), with a separate review record.
- `quotation_decision`, `quotation_acceptance_terms` and `request_rejection` have no ER counterpart.
- The database enforces the minimum-price rule and the deposit and PO rules as table checks.

### Jobs and production

- `job` has one row per quotation (unique `quotation_id`); the ER allowed many jobs per quotation.
- `estimated_cost`, `estimated_time`, `actual_cost` and `actual_time` are not stored on `job`. Actual labor is in `job_time_entry`, and material cost comes from `job_material_usage`. Estimates were not found in the migrations.
- `job` adds `deposit_amount`, `balance`, evidence references, `due_at` and `material_plan_version`.
- `job_material_usage` stores `unit_cost` and a generated `total_amount`, not just `qty_used`.
- `job_material_requirement` (the plan) and `job_material_command` (idempotent command audit) are new.
- `material_reservation` keeps the ER's reservation idea, with the status derived from quantity. Reservations never consume physical stock.
- `production_event`, `production_return`, `job_time_entry` and `fulfillment_event` are new.

### Stock and procurement

- The ER's single `STOCK_MOVEMENT` table with a movement-type ENUM is split. `stock_movement` only records admin additions and count corrections.
- Supplier receiving is recorded in `material_receipt` and `material_receipt_line`.
- Job issue is recorded in `production_event` and `job_material_usage`. There is no `RETURN` movement type.
- `material_order.status` holds only `ordered` or `cancelled`. Receiving progress is derived from receipts; `partially_received` and `received` are not stored.
- `payment_status`, `payment_method`, `payment_proof_url` and `paid_at` are not on `material_order`. Supplier payment is in `supplier_settlement`.
- `material_order_job` links an order to zero or more jobs. It is not in the ER.
- `material_order_item` snapshots `material_name` and `unit_name`. Received and damaged quantities are not stored on the item; they are derived from receipt lines and claims.
- `material_order_event` and `material_receipt` are immutable command and receipt records.
- Defect handling has no `WRITTEN_OFF` status, matching the replacement-only rule.

### Delivery and billing

- `delivery` has `shipping`, `delivered` and `returned` statuses. The ER's `preparing` status is not present. Shipping cost and proof URL were not found in the migrations.
- `invoice` has `amount`, `credit_days`, `issued_at` and `due_at`. It does not store a status (unpaid, partial, paid); that is derived from payments.
- `customer_payment` replaces `PAYMENT` and adds approval status and evidence.

### Money and types

- The ER uses `double` for money. The implementation uses `NUMERIC`: `(18,2)` for THB amounts and `(18,6)` for quantities and per-base-unit costs.
- Timestamps are `timestamptz`.

## Tables with no ER counterpart

Authentication and sessions: `account`, `auth_session`.
Customer access: `customer_member`, `customer_invitation`.
Notifications and events: `notification`, `outbox`.
Evidence staging and private files: `request_attachment`, `order_evidence`, `customer_evidence`, `term_evidence`.
Documents: `shop_profile`, `customer_po`.
Audit and idempotency: `material_catalog_audit`, `supplier_audit`, `material_order_event`, `job_material_command`, `fulfillment_event`, `uc4_cancellation`.
Demo data: `demo_seed_dataset`, `demo_seed_record`, `demo_seed_step`, `demo_workflow_run`, `demo_workflow_step`.

## Not verified

These were not confirmed by reading the migrations and should be checked with a schema dump of a migrated local database (`pg_dump --schema-only`):

- Exact columns of `admin` and `worker`, including whether a status column exists.
- Shipping cost and proof fields on `delivery`.
- Whether `tax_id` sits on `customer` itself.
- Whether any `estimated_*` columns exist on `job`.

## Update: ER alignment phase 1 (migration 000023)

Three tables were renamed to their ER names and one was split. Behaviour, constraints and data are unchanged.

| ER entity | Table now | Was |
| --- | --- | --- |
| `PAYMENT` | `payment` | `customer_payment` |
| `ADDRESS` | `address` | `customer_address` (also gained `edited_at`) |
| `MATERIAL_DEFECT_LOG` | `material_defect_log` | `material_claim` |
| `DEPOSIT` | `deposit` | `term_evidence` where `kind='deposit'` (also gained `percent`) |
| `PURCHASE_ORDER` | `purchase_order` | `term_evidence` where `kind='po'` |

`term_evidence` remains as a read-only union view of `deposit` and `purchase_order`. `material_reservation` gained an `id`. The attribute differences listed above (stored statuses, estimated and actual job fields, payment columns and so on) are unchanged and are the scope of later phases.

## Update: ER alignment phase 2 (migration 000024)

These ER attributes are now stored columns. The database keeps each one equal to the value computed from the source tables, and its history guard rejects any other value, so it cannot drift.

| ER attribute | Column | How it is kept in sync |
| --- | --- | --- |
| `INVOICE.status` | `invoice.status` (`unpaid`, `partial`, `paid`) | Trigger on `payment_allocation` insert |
| `MATERIAL_ORDER_ITEM.qty_received`, `qty_damaged`, `status`, `received_at` | same names on `material_order_item` | Trigger on `material_receipt_line` insert and on order cancellation. `qty_received` is good plus damaged; `status` is `received`, `damaged`, `cancelled` or `ordered` |
| `MATERIAL_ORDER_ITEM.expected_at`, `created_at` | same names | Set when the item is inserted (`created_at` equals the order time because both use the transaction time) |
| `MATERIAL_ORDER.payment_method`, `paid_at`, `payment_proof_url` | same names on `material_order` | Copied from the supplier settlement when it is recorded; `payment_proof_url` holds the private object key, never a URL |
| `JOB.estimated_cost` | `job.estimated_cost` | Quotation material plus labor cost, set when the job is created |
| `JOB.actual_cost` | `job.actual_cost` | Actual labor plus issued material, set when the job completes and cleared if a return reopens it |
| `JOB.edited_at` | `job.edited_at` | Set on every update |

Not added: `JOB.estimated_time` and `JOB.actual_time` (no stored source; the ER types them as timestamps but the names read as durations), `MATERIAL_ORDER_ITEM.edited_at` stays NULL until an item first changes, and the item statuses `preparing` and `in_transit` have no source. `worker.status` and `admin.status` are still derived from `account.is_active`.
