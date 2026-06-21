# UPD HTTP 400 Diagnostic Report v0.1

Date: 2026-06-21  
Project: `D:\Projects\freight-platform`  
Git baseline: `f0f3e3b` (snapshot commit; diagnostic work uncommitted)

---

## Summary

Integration smoke test failed at the UPD step with **HTTP 400** on Windows/Git Bash.  
Backend billing-register-service is **correct**. The failure was caused by **how the smoke test sent the JSON body on Windows**, not by billing business rules or a wrong API contract.

After sending UTF-8 JSON via `printf | curl --data-binary @-` and building `function_code` with `jq` Unicode escapes, UPD returns **HTTP 201** and the full smoke test passes.

---

## UPD endpoint

| Field | Value |
| ----- | ----- |
| Service | `billing-register-service` (direct in smoke test) |
| Method | `POST` |
| URL | `http://localhost:8087/v1/billing-registers/{register_id}/upd` |
| Gateway equivalent | `POST /api/v1/billing-registers/{id}/upd` → proxied to same path |
| Handler | `ClosingDocumentHandler.CreateUPD` |
| Service | `ClosingDocumentService.CreateUPD` |
| Repository | `ClosingDocumentRepository.CreateUPD` |
| Expected success status | **201 Created** |

### Backend validation rules (no business change in this pack)

| Rule | Detail |
| ---- | ------ |
| Register status | Must be `APPROVED` or `CLOSING_DOCUMENTS_CREATED` (`ValidateCreateClosingDocumentRegisterStatus`) |
| Billing items | Not explicitly validated at UPD time; totals come from register row after `calculate` |
| Shipment status | Not checked by billing-register-service for UPD |
| Invoice / act / VAT invoice | **Not required** before UPD |
| Required JSON fields | `tenant_id`, `upd_number`, `upd_date`, `seller_company_id`, `buyer_company_id`, `function_code` |
| Allowed `function_code` | `СЧФ`, `СЧФДОП`, `ДОП` (Cyrillic; DB CHECK constraint matches) |
| Amounts in payload | **Not sent** — copied from register: `total_without_vat`, `vat_rate`, `vat_amount`, `total_with_vat` |
| After success | Register status → `CLOSING_DOCUMENTS_CREATED` |

---

## Current smoke-test request

From `tests/integration/smoke-test.sh` (after diagnostic pack):

```http
POST /v1/billing-registers/{BILLING_REGISTER_ID}/upd
Content-Type: application/json; charset=utf-8

{
  "tenant_id": "<TENANT_ID>",
  "upd_number": "UPD-TEST-<SMOKE_RUN_ID>",
  "upd_date": "2026-07-16",
  "seller_company_id": "<CARRIER_COMPANY_ID>",
  "buyer_company_id": "<SHIPPER_COMPANY_ID>",
  "function_code": "СЧФДОП"
}
```

Transport: `printf '%s' "$UPD_PAYLOAD" | curl --data-binary @-`  
Payload built with `jq` and `\u0421\u0427\u0424\u0414\u041e\u041f` for `function_code`.

`tests/integration/full-flow-smoke-test.sh` is an alias — it runs the same `smoke-test.sh`.

---

## Actual error response

### Legacy behavior (reproduced 2026-06-21)

Command: `tests/integration/run-upd-400-repro.sh` → Repro A (`curl -d "$DATA"` with Cyrillic heredoc)

**HTTP 400**

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "invalid function_code",
    "details": {
      "field": "function_code",
      "value": "������"
    }
  }
}
```

### Fixed transport (Repro B)

Same payload via `printf | curl --data-binary @-` → **HTTP 201**, UPD created with `"function_code":"СЧФДОП"`.

### Other possible 400 messages (not the primary smoke failure)

| Message | When |
| ------- | ---- |
| `invalid JSON body` | Malformed JSON / corrupted multiline `-d` body |
| `documents can only be created for APPROVED or CLOSING_DOCUMENTS_CREATED register` | Register not approved (Variant B) |
| `referenced record does not exist` | Invalid UUID references (Variant A payload) |

---

## Billing register state before UPD

Example from smoke test run `TEST-20260621163404`:

| Field | Value |
| ----- | ----- |
| `billing_register_id` | `83e7f139-1fd1-4a56-8d2c-b350e6052b33` |
| `tenant_id` | `91babc18-1fe0-4df3-8d2c-b350e6052b33` |
| `shipment_id` | `24235d34-6c8f-4beb-a0c8-8e0039cb7455` |
| `register_status_before_upd` | `APPROVED` |
| `register_items_count` | `1` |
| `calculate_response` | `{"status":"CALCULATED","total_without_vat":105000,"vat_amount":21000,"total_with_vat":126000}` |
| `approve_response` | `{"status":"APPROVED","approved_by":"...","approved_at":"2026-06-21T13:34:18Z"}` |

Flow order in smoke test is **complete** (companies → user → transport order → RFx → bid → shipment → READY_FOR_BILLING → billing register → item → calculate → approve → UPD).

---

## Root cause

**Variant A — smoke test payload transport on Windows**

1. Smoke test called `curl -d "$data"` with a heredoc containing Cyrillic `function_code`.
2. On Windows/Git Bash, passing multiline UTF-8 JSON as a **command-line argument** to `curl.exe` corrupts bytes before they reach billing-register-service.
3. Server receives garbled `function_code`, fails `ValidateCreateUPDInput`, returns HTTP 400 `invalid function_code`.

**Not the cause:** Variants B–F (register timing, empty register, shipment status, backend validation bug, gateway routing). Register was `APPROVED`, had items and totals; direct call to `:8087` with correct UTF-8 succeeds.

---

## Minimal fix plan

1. **Keep** `printf | curl --data-binary @-` in `api_request()` (already applied).
2. **Keep** UPD payload via `jq` with Unicode escapes for `function_code` (already applied).
3. **Add** UPD diagnostic block before create (done in this pack).
4. **Improve** error logging on non-2xx (print labeled response body; done).
5. **Do not change** billing-register-service validation or API contract.
6. Optional: document repro scripts under `tests/integration/run-upd-400-repro.sh` for future Windows regressions.

---

## Files that likely need changes

| File | Change | Status |
| ---- | ------ | ------ |
| `tests/integration/smoke-test.sh` | stdin curl, jq payload, diagnostics, error body | **Updated** |
| `tests/integration/run-upd-400-repro.sh` | Repro helper | **Added** |
| `tests/integration/upd-400-repro.sh` | A/B repro | **Added** |
| `services/billing-register-service/**` | None | No change |
| `services/api-gateway/**` | None | No change |

---

## Risk level

**Low** — test infrastructure only. No billing business logic or API contract changes.

---

## Recommended next command

Verify end-to-end after diagnostic changes:

```powershell
cd D:\Projects\freight-platform
& "C:\Program Files\Git\bin\bash.exe" -lc "make integration-smoke-test"
```

Optional repro of legacy failure:

```powershell
& "C:\Program Files\Git\bin\bash.exe" "D:/Projects/freight-platform/tests/integration/run-upd-400-repro.sh"
```

When satisfied, create a follow-up commit (not part of this pack):

```powershell
git add tests/integration/smoke-test.sh tests/integration/upd-400-repro.sh tests/integration/run-upd-400-repro.sh docs/UPD_HTTP_400_DIAGNOSTIC_V0.1.md
git commit -m "test: diagnose and fix UPD HTTP 400 on Windows smoke test"
```
