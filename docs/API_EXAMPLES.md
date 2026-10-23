# API examples (PowerShell)

The canonical route and permission index is [openapi.yaml](openapi.yaml). Feature guides (`UC1.md` … `UC9.md`) define detailed workflow rules. These examples use the local API default; set `$api` to the address you run. No example contains a real account or secret.

## Sign in

```powershell
$api = 'http://localhost:8080'
$loginBody = @{ username = 'demo-customer'; password = 'replace-with-local-password' } | ConvertTo-Json
$login = Invoke-RestMethod -Method Post -Uri "$api/auth/login" -ContentType 'application/json' -Body $loginBody
$headers = @{ Authorization = "Bearer $($login.access_token)" }
```

`access_token` is an opaque revocable session token. Keep it out of source control and logs. Customer registration uses `POST /auth/register`; new company registration requires `customer_type = company` and `company_name`. Joining an existing company uses `customer_type = company` plus `invitation`, and omits `company_name`.

## Submit a request

Choose a `customer_id` from `GET /customers`. The server checks current membership; it does not infer access from a company name.

```powershell
$requestBody = @{
  customer_id = '00000000-0000-4000-8000-000000000001'
  description = 'Rubber compression mold insert'
  qty = 2
  material_spec = 'Customer supplied drawing, revision A'
} | ConvertTo-Json
$request = Invoke-RestMethod -Method Post -Uri "$api/requests" -Headers $headers -ContentType 'application/json' -Body $requestBody
$request.id
```

## Upload a private drawing

The API returns a short-lived signed URL. Upload bytes directly to that URL with exactly the returned headers, then call the API completion route. The API validates content and freezes the object before it becomes ready. Never copy a signed URL into a public issue or document.

```powershell
$intent = @{ file_name = 'drawing.pdf'; content_type = 'application/pdf'; byte_size = (Get-Item '.\drawing.pdf').Length } | ConvertTo-Json
$upload = Invoke-RestMethod -Method Post -Uri "$api/requests/$($request.id)/uploads" -Headers $headers -ContentType 'application/json' -Body $intent
Invoke-WebRequest -Method Put -Uri $upload.url -Headers $upload.headers -InFile '.\drawing.pdf' -ContentType 'application/pdf' | Out-Null
$attachment = Invoke-RestMethod -Method Post -Uri "$api/uploads/$($upload.attachment_id)/complete" -Headers $headers
$attachment.state
```

The exact upload allowlist and 100 MB limit are implemented in `internal/file`. Evidence is private; download routes return short-lived signed links only after authorization. Request attachments allow authorized staff and current company members; customer upload completion is uploader-scoped.

## Worker quotation example

Quotation cost/price amounts are JSON decimal strings. This quote meets the confirmed minimum of `(material + labor) * 1.60`, with the minimum rounded upward to one satang.

```powershell
$quoteBody = @{ material_cost = '100.00'; labor_cost = '50.00'; final_price = '240.00' } | ConvertTo-Json
$quote = Invoke-RestMethod -Method Post -Uri "$api/requests/$($request.id)/quotations" -Headers $headers -ContentType 'application/json' -Body $quoteBody
$sendBody = @{ expected_lock_version = $quote.lock_version } | ConvertTo-Json
$quote = Invoke-RestMethod -Method Post -Uri "$api/quotations/$($quote.id)/send" -Headers $headers -ContentType 'application/json' -Body $sendBody
```

Use a Worker token for quotation creation and sending. A company customer decides with `POST /quotations/{quotation_id}/decision`, body `{ outcome = 'accept', expected_lock_version = <current quotation lock_version> }`; rejection uses `outcome = 'reject'` and a `reason`. Decisions require a currently active company membership.

## Read fulfillment or submit payment

Protected job commands use `idempotency_key` (UUID) and `expected_version` from the latest fulfillment response. Retry a timed-out command with the same key and identical body. A Worker sees invoice/balance information but not payment receipt rows or proof.

```powershell
$fulfillment = Invoke-RestMethod -Method Get -Uri "$api/fulfillment/jobs/$jobId" -Headers $headers
$command = @{ idempotency_key = [guid]::NewGuid().ToString(); expected_version = $fulfillment.version }
# Admin/Worker invoice command (only after production completion and terms):
$body = $command | ConvertTo-Json
Invoke-RestMethod -Method Post -Uri "$api/fulfillment/jobs/$jobId/invoice" -Headers $headers -ContentType 'application/json' -Body $body
```

For a customer bank installment, first upload a private proof through `/fulfillment/jobs/{job_id}/evidence/uploads`, complete it, then POST `/fulfillment/jobs/{job_id}/payments/bank` with `idempotency_key`, current `expected_version`, `amount` as a decimal string, `paid_at` as RFC3339 UTC, and `evidence_id`. A pending submission does not reduce the balance; an Admin must approve it.

## Error handling

Errors use `{ "error": { "code": "...", "message": "..." } }`. Common statuses are 400 invalid input, 401 missing/invalid session, 403 role or permission denied, 404 missing or deliberately hidden inaccessible object, 409 stale state/idempotency conflict, and 500 unexpected server failure. Reload current state after 409; do not invent a new retry key to repeat a command whose outcome is unknown.
