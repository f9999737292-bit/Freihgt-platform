# Low-code Conditional Required Validation v0.1

## Summary

`PUT /v1/low-code/custom-field-values` now enforces conditional required rules from `validation_rule_json` (`if` / `then.required`), aligned with the preview renderer.

## Rule DSL

Same as preview:

```json
{ "if": { "field": "cargo_class", "in": ["A", "B", "C"] }, "then": { "required": ["loading_window_note"] } }
```

Supports field conditions (`equals`, `in`, `not_in`) and optional context conditions when `validation_context` is sent:

```json
{ "validation_context": { "entity_status": "APPROVED", "role": "PLATFORM_ADMIN" } }
```

## Server flow

1. Validate each submitted field (type, static required, simple rules)
2. Merge existing DB values with incoming PUT payload
3. Evaluate conditional required rules against merged snapshot
4. Reject with `VALIDATION_RULE_FAILED` when a conditionally required field is empty

**Code:** `services/low-code-service/internal/domain/conditional_required.go`

## Frontend

`LowCodeCustomFieldsPanel` passes `validation_context` from preview context on save.

## Verification

```powershell
make seed-lowcode-demo
```

1. Open **DEMO-TO-001** → Edit → set `cargo_class` to **A** → clear `loading_window_note` → Save → expect validation error
2. Fill `loading_window_note` → Save → success
3. Set `cargo_class` to **GENERAL** → Save without note → success

## Limitations

- Field-based rules only unless client sends `validation_context`
- No visibility enforcement on save
- Not a full rule engine (`then.visible`, cross-template rules)

## Next action

1. Entity reference / FILE upload editors
2. RFx lot / bid entity types when templates are needed

See also: `docs/LOW_CODE_PREVIEW_CONDITIONAL_REQUIRED_V0.1.md`, `docs/LOW_CODE_CREATE_FIRST_VALUE_EDIT_V0.1.md`.
