# Source Report Traceability

This index connects the supplied Systems Analysis and Design report to the current backend. Use the numbered UC specifications and migration files for implementation details; do not copy the report's historical SQL directly into the application.

## Use Case map

| Source UC | Source topic | Current backend contract | Notes |
| --- | --- | --- | --- |
| UC1 | Customer registration and request details | [001-UC1.md](001-UC1.md), [000-auth.md](000-auth.md) | Customer self-registration, active company membership, scoped requests, and private request files. |
| UC2 | Request review and quotation | [002-UC2.md](002-UC2.md) | Worker quotation actions; Admin reads are role-limited; internal cost is redacted from Customers. |
| UC3 | Customer quotation decision and terms | [003-UC3.md](003-UC3.md) | Five rejections permit revision; the sixth cancels; accepted terms are immutable. |
| UC4 | Deposit/PO evidence and job creation | [004-UC4.md](004-UC4.md) | Private evidence, staff review, immutable deadline, and one queued job after all required approvals. |
| UC5 | Stock, material plans, reservations | [005-UC5.md](005-UC5.md) | Exact stock and reservation rules; explicit per-material conversion factors only. |
| UC6 | Supplier procurement order | [006-UC6.md](006-UC6.md) | Admin-only; order/job references are flexible; no physical stock effect. |
| UC7 | Receiving, defects, supplier settlement | [007-UC7.md](007-UC7.md) | Partial good/damaged receipts, replacement-only claims, explicit credit terms, one full settlement. |
| UC8 | Production and material usage | [008-UC8.md](008-UC8.md) | Worker execution, Admin cost, actual usage/time, and unused-reservation release. |
| UC9 | Delivery, invoice, payment, rework | [009-UC9.md](009-UC9.md) | Customer inspection and bank evidence; Admin review/cash; accepted plus fully paid closes. |
| UC10 | Unit and unit-conversion maintenance | [005-UC5.md](005-UC5.md) | Unit management and per-material conversion factors exist within UC5; entering quantities in other units is not yet supported. |
| UC11 | Worker registration | [000-auth.md](000-auth.md) | Admin staff management exists with current required-contact rules; do not use the report's SQL or duplicate-matching behavior. |
| UC12 | Admin registration through a super-admin | [000-auth.md](000-auth.md) | No public Admin-registration use case or super-admin hierarchy. First Admin is provisioned by the private bootstrap command. |

## Business-process traceability

| Report process segment | Current backend coverage |
| --- | --- |
| Receive customer request and attachments | UC1 |
| Review/quote and customer decision | UC2–UC3 |
| Collect PO/deposit and open a job | UC4 |
| Check stock and reserve material | UC5 |
| Order from supplier | UC6 |
| Receive good/damaged material and settle supplier | UC7 |
| Execute production and reconcile actual use | UC8 |
| Deliver, invoice, inspect, collect payment, and close | UC9 |

## Legacy entity names in the academic report

The report lists these conceptual entities. This is a glossary for tracing report sections, not a declaration that identically named PostgreSQL tables exist today.

| Source entities | Current contract area |
| --- | --- |
| `PERSON`, `CUSTOMER`, `WORKER`, `ADMIN`, `ADDRESS` | Authentication, staff administration, customer identity, and company membership (`000-auth.md`, `001-UC1.md`). |
| `JOB_REQUEST`, `REQUEST_ATTACHMENT` | Customer requests and private attachments (`001-UC1.md`). |
| `QUOTATION`, `QUOTATION_COST` | Quote lifecycle and customer-safe snapshots (`002-UC2.md`, `003-UC3.md`). |
| `PURCHASE_ORDER`, `DEPOSIT` | PO/deposit term evidence and approval (`004-UC4.md`). |
| `JOB`, `JOB_PHOTO`, `JOB_MATERIAL_USAGE`, `MATERIAL_RESERVATION` | Job opening, plans/reservations, production usage, and private photos (`004-UC4.md`, `005-UC5.md`, `008-UC8.md`). |
| `MATERIAL`, `UNIT`, `UNIT_CONVERSION`, `STOCK_MOVEMENT` | Catalog, stock ledger, and reservation/usage rules (`005-UC5.md`, `007-UC7.md`, `008-UC8.md`). Per-material conversion factors are stored; stock stays in the base unit. |
| `SUPPLIER`, `MATERIAL_ORDER`, `MATERIAL_ORDER_ITEM`, `MATERIAL_DEFECT_LOG` | Supplier orders, receiving, replacement claims, and settlement (`006-UC6.md`, `007-UC7.md`). |
| `DELIVERY`, `INVOICE`, `PAYMENT`, `PAYMENT_ALLOCATION` | Delivery/inspection, invoices, customer payment evidence, allocations, and closure (`009-UC9.md`). |

Current normalized schema and keys are defined by `migrations/`; use those migrations and repository SQL rather than assuming the report's entity names map one-to-one to current tables.

## SQL and diagram handling

- The report's query table is a design artifact. It includes schema and status assumptions that predate the implemented UC contracts, plus MySQL-specific idioms (for example `ROW_COUNT()` and a comment about MySQL `UPDATE` evaluation). It is not PostgreSQL DDL/API documentation and must not be run against a current database.
- The report's ER diagram and query annotations are historical. Current cardinalities, audit tables, session rules, evidence privacy, and exact status transitions are defined by the migrations and UC docs.
- The source Markdown embeds diagram images as base64. This repository keeps this text-based traceability summary rather than duplicating large opaque image payloads; consult the owner's original report for its diagrams.

## Naming convention

New Markdown documentation in `docs/` uses a three-digit numeric prefix plus a descriptive text slug, for example `020-USECASE10.md`. Existing filenames such as `001-UC1.md` are retained to preserve links and history; do not rename them as part of this source mapping.
