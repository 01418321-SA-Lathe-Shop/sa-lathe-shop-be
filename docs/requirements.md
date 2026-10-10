# Requirements baseline

Sources: owner's conversation and Mermaid schema attachment at
C:/Users/win/.codex/attachments/77652150-d9dc-4592-a38b-72deefcdca2b/Pasted text.txt.
UC_Description_Detailed_Exception_Integrated_v2.docx was fully read earlier in
this conversation; it is no longer available at its original Downloads path.
No current sequence document is supplied. Confirm current originals before
feature coding. The final stack and clarifications from the owner override
the older schema's float money fields and public URL approach.

UC1 request and private drawings; UC2 estimate and quote; UC3 customer accepts
or rejects and terms are computed; UC4 deposit/company-issued PO and job opening;
UC5 required materials, stock check and reservation; UC6 supplier orders;
UC7 partial receipts, damage and stock/cost updates; UC8 production/material
usage/photos; UC9 delivery/invoice/payment, with payment after completion allowed.

Company threshold: <= 10,000 PO/no deposit; > 10,000 PO/50% deposit.
Individuals pay 50% first, no PO. No agreed deposit/PO expiry. Profit target
60%, calculation basis unresolved. Count rejection events rather than drafts;
confirm more-than-five threshold. Supplier usually settles in one payment.
Track delivery method self/third-party, not external logistics integration.

Plan requirement quantities separately from quantities reserved and consumed.
Partial stock must not hide shortages. Stock and stock movements change in one
transaction with row locks; reconcile unconsumed reservations. Damaged stock
does not enter usable balance. MATERIAL_ORDER_ITEM needs partially_received.
Use NUMERIC for all money and exact quantities. Files are private S3 objects;
DB stores object keys/metadata, never bytes or permanent signed URLs.
