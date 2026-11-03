# Printed document details

Data that the frontend prints on the Quotation, the customer's Purchase Order and the Thai job contract. Migration `000017_documents` adds it. Every text field may be empty; an empty value means "not provided", and the printed document shows a fill-in line instead. No value is invented or defaulted.

## Shop profile
- `GET /shop-profile`: any signed-in role. Before the first save it returns empty strings and `lock_version` 0 with no `updated_at`.
- `PUT /shop-profile`: Admin only. Body `{name, address, tax_id, phone, email, bank_name, bank_account_name, bank_account_no, expected_lock_version}`.
  - `expected_lock_version` 0 creates the single row; later saves must send the stored version, otherwise the API returns 409.
- The table starts empty on purpose: the demo seed requires every application table to be empty.
- There is no logo upload. Attachments belong to a job request, so a shop logo would need its own storage decision.

## Customer billing details
- `GET /customers/{customer_id}/billing`: members of that customer, plus Admin and Worker (staff print quotations and contracts). Other customers get 404.
- `PUT /customers/{customer_id}/billing`: the customer's owner member only (403 for other members). Body `{address, tax_id, phone, email, expected_lock_version}`.
- Stored as columns on `customer` with `billing_lock_version`.

## Quotation document fields
- Draft create and edit accept optional `valid_until` (`YYYY-MM-DD` or null) and `notes` (up to 2,000 characters). Both are returned with the quotation, including to the customer.
- An edit that omits them clears them, as with the other draft fields. Who may write quotations, and the pricing rule, are unchanged.

## Customer Purchase Order
- `GET /quotations/{quotation_id}/customer-po`: anyone who can read the request; 404 until the customer writes one.
- `PUT /quotations/{quotation_id}/customer-po`: a member of the requesting customer. Body `{po_no, issued_on (YYYY-MM-DD), remarks, expected_lock_version}`; 0 creates.
  - Allowed only while the accepted terms require a PO (`po_required`) and no PO evidence has been approved; otherwise 409.
  - The PO amount is not stored. It is always the accepted quotation's `final_price`.
- Writing the PO does not submit it. The customer still uploads the signed PO through the existing evidence flow, and staff review that upload as before.

## Verification
- `go build`, `go vet` and `go test ./...` pass, including service tests for roles and validation.
- On the local demo database, 000017 ran up, down and up.
- Real HTTP checks covered: role checks, cross-customer 404s, owner-only billing, PO creation blocked when no PO is required or the PO is already approved, duplicate create and stale version (409), and staff reads.
- Repository integration suites were not run against a disposable database.
