# Connecting Cursor without turning the autopilot on

Do this only after independent QA of the autopilot foundation and a separate controller approval. This phase does not create a Cursor Automation and does not call the API.

## One dispatcher

GitHub Actions is the only launcher. A Cursor Automation that also calls Create Agent would race the claim lock and can double-spend. If a Cursor Automation is added later, limit it to a daily digest that does not create agents. The in-app Automations editor is the place to do that, after approval. Do not enable it from this repository change.

## API keys

1. Open Cursor Dashboard → API Keys and create a user or service-account key for Cloud Agents. Team Admin API keys are a different credential and are not a substitute for the Cloud Agents key.
2. Create an Admin API key only if this team can call `POST /teams/filtered-usage-events`. Without it the dispatcher fail-closes and launches nothing.
3. Store both values as GitHub Actions secrets. Names and steps are in `GITHUB_SECRETS.md`.
4. Do not put either key in the repository, in an issue, or in `envVars` of a cloud agent.

## Repository access

The key's Cursor user or service account must be able to open pull requests in `f9999737292-bit/Freihgt-platform`. The dispatcher sends `repos[0].url` as `https://github.com/f9999737292-bit/Freihgt-platform`. It does not call `GET /v1/repositories`.

## Model

The create request omits `model`. Cursor then uses the documented default chain: user default, then team default, then system default. Set the default in the Cursor dashboard before any trial. Do not hardcode a model id in this repository until `GET /v1/models` has been checked for that account.

## Trial shape

Keep `max_parallel_agents` at 3 or lower. For the first trial, leave the daily cap at 8 and approve one `READY` issue only. Confirm the claim ref, the `autopilot/<task-id>` branch, the draft PR, and a zero merge to `main` before approving a second issue.

## Official references

- https://cursor.com/docs/cloud-agent/api/overview
- https://cursor.com/docs/cloud-agent/api/endpoints
- https://cursor.com/docs/account/teams/admin-api
- https://cursor.com/docs/sdk/typescript for teams that later choose `@cursor/sdk` instead of raw HTTP. This phase uses the HTTP surface so the GitHub runner does not depend on an SDK package.
