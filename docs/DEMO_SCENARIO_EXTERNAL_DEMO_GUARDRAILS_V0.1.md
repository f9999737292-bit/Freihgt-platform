# Demo Scenario External Demo Guardrails v0.1

## Summary

Guardrails for using the current production UI in a controlled external or investor-style demonstration.

Base commit: `f929fde`.

## Allowed Demo Positioning

```text
Controlled static UI walkthrough.
```

## Required Opening Disclaimer

```text
This demo shows the current production static UI and product direction.
Live-data workflows and authenticated role scenarios are not signed off yet.
Full production readiness is not claimed.
```

## Safe To Show

| Area                    | Rule                                             |
| ----------------------- | ------------------------------------------------ |
| root/login              | safe                                             |
| login cleanliness       | safe                                             |
| backend status online   | safe as technical confidence                     |
| product route structure | safe as concept/static walkthrough               |
| RBAC/product concept    | safe with caveat                                 |
| /health                 | technical only, not business audience by default |

## Avoid Showing

| Area                            | Reason              |
| ------------------------------- | ------------------- |
| real credentials                | not approved        |
| fake production sessions        | not approved        |
| cookies/localStorage/JWT        | secret/session risk |
| authenticated workflows         | not signed off      |
| live data promises              | not signed off      |
| operational SLA/security claims | not signed off      |

## Phrases To Use

```text
"Это контролируемая демонстрация статического production UI."
"Живые данные и авторизованные сценарии будут подтверждаться отдельным этапом."
"Сейчас показываем продуктовую структуру, навигацию и визуальную готовность интерфейса."
```

## Phrases To Avoid

```text
"Платформа полностью production-ready."
"Все backend/API сценарии готовы."
"Можно запускать клиентов в продуктив."
"Юридический/биллинг/E2E контур полностью готов."
```

## Decision

```text
DEMO_SCENARIO_EXTERNAL_DEMO_GUARDRAILS_CREATED
```
