# Review Protocol

Independent review gate before integration.

## Reviewer

Use the `reviewer` subagent (readonly).

## Inputs required

- Agent Task (allowed/forbidden paths, goal)
- Handoff document with SHAs and verification results
- Full diff: `git diff <base-sha>..<final-sha>`

## Inspection checklist

- [ ] Requirements met
- [ ] Diff matches Allowed Paths only
- [ ] No out-of-scope edits
- [ ] Architecture / service boundaries respected
- [ ] Security and tenant isolation (invoke `security-auditor` when relevant)
- [ ] Tests claimed were actually run or marked NOT_RUN
- [ ] OpenAPI updated when public API changed
- [ ] Migrations synchronized with repository/model changes
- [ ] Backward compatibility preserved unless authorized

## Verdicts

| Verdict | Meaning |
|---------|---------|
| **PASS** | Ready for integration |
| **PASS_WITH_NOTES** | Accept with documented follow-ups |
| **FAIL** | Must fix before integration |
| **BLOCKED** | Missing inputs, environment, or dependency |

## Rules

- Reviewer does not repair implementation.
- Do not accept handoff claims without diff evidence.
- Security-sensitive changes require security auditor findings attached.

## Output

- Verdict
- Findings by severity
- Required fixes (if FAIL)
- Approved SHA for integration (if PASS)
