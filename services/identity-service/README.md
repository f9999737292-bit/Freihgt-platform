# identity-service



Authentication and identity management service for the freight platform.



## Responsibilities



- User CRUD (`core.users`)

- Login and JWT access tokens

- Role assignment (`core.user_roles`)

- Role and permission lookup (`core.roles`, `core.permissions`)

- User-company memberships lookup (`core.company_memberships`)

- Company-scoped role assignment for members



## Environment variables



| Variable | Default | Description |

|----------|---------|-------------|

| `IDENTITY_SERVICE_PORT` | `8081` | HTTP port |

| `DATABASE_URL` | local postgres URL | PostgreSQL connection string |

| `JWT_SECRET` | `dev_secret_change_me` | JWT signing secret |

| `JWT_ACCESS_TOKEN_TTL_MINUTES` | `60` | Access token lifetime |

| `LOG_LEVEL` | `info` | Log level |

| `ENVIRONMENT` | `development` | Runtime environment |



## Run locally



From monorepo root:



```bash

make run-identity-service

```



Or from this directory:



```bash

cp .env.example .env

go run ./cmd/server

```



Requires PostgreSQL from `make dev-up` and applied migrations.



## Endpoints



| Method | Path | Description |

|--------|------|-------------|

| GET | `/health` | Health check |

| POST | `/v1/users` | Create user |

| GET | `/v1/users/{id}` | Get user |

| GET | `/v1/users` | List users |

| PATCH | `/v1/users/{id}` | Update user |

| DELETE | `/v1/users/{id}` | Soft delete user |

| POST | `/v1/auth/login` | Login |

| GET | `/v1/auth/me` | Current user (Bearer token) |

| POST | `/v1/users/{id}/roles` | Assign role |

| GET | `/v1/users/{id}/roles` | List user roles |

| GET | `/v1/users/{user_id}/companies` | List user companies |

| POST | `/v1/users/{user_id}/companies/{company_id}/roles` | Assign role in company |

| DELETE | `/v1/users/{user_id}/companies/{company_id}/roles/{role_id}` | Remove role in company |

| GET | `/v1/roles` | List system and tenant roles |

| GET | `/v1/roles/{role_id}/permissions` | List role permissions |



## Examples



### 1. Create a tenant (one-time for local dev)



```bash

docker exec -i freight_postgres psql -U freight -d freight_platform -c \

  "INSERT INTO core.tenants (code, name) VALUES ('demo', 'Demo Tenant') ON CONFLICT (code) DO NOTHING RETURNING id;"

```



Save the returned `id` as `TENANT_ID`.



### 2. Create a company (company-service on port 8082)



```bash

curl -X POST http://localhost:8082/v1/companies \

  -H "Content-Type: application/json" \

  -d '{

    "tenant_id": "TENANT_ID",

    "legal_name": "ООО Ромашка",

    "short_name": "Ромашка",

    "company_type": "SHIPPER",

    "country_code": "RU",

    "preferred_locale": "ru-RU"

  }'

```



Save the returned `id` as `COMPANY_ID`.



### 3. Create a user



```bash

curl -X POST http://localhost:8081/v1/users \

  -H "Content-Type: application/json" \

  -d '{

    "tenant_id": "TENANT_ID",

    "email": "user@example.com",

    "phone": "+79990000000",

    "password": "StrongPassword123!",

    "full_name": "Иван Иванов",

    "preferred_locale": "ru-RU"

  }'

```



Save the returned `id` as `USER_ID`.



### 4. Add user to company (company-service)



```bash

curl -X POST http://localhost:8082/v1/companies/COMPANY_ID/members \

  -H "Content-Type: application/json" \

  -d '{

    "tenant_id": "TENANT_ID",

    "user_id": "USER_ID",

    "position": "Логист"

  }'

```



### 5. List company members (company-service)



```bash

curl "http://localhost:8082/v1/companies/COMPANY_ID/members?tenant_id=TENANT_ID&status=ACTIVE"

```



### 6. Get user companies



```bash

curl "http://localhost:8081/v1/users/USER_ID/companies?tenant_id=TENANT_ID&status=ACTIVE"

```



### 7. Assign role to user in company



Get a role id first:



```bash

curl "http://localhost:8081/v1/roles?tenant_id=TENANT_ID"

```



Then assign:



```bash

curl -X POST http://localhost:8081/v1/users/USER_ID/companies/COMPANY_ID/roles \

  -H "Content-Type: application/json" \

  -d '{

    "tenant_id": "TENANT_ID",

    "role_id": "ROLE_ID"

  }'

```



Remove role:



```bash

curl -X DELETE "http://localhost:8081/v1/users/USER_ID/companies/COMPANY_ID/roles/ROLE_ID?tenant_id=TENANT_ID"

```



Login:



```bash

curl -X POST http://localhost:8081/v1/auth/login \

  -H "Content-Type: application/json" \

  -d '{

    "tenant_id": "TENANT_ID",

    "email": "user@example.com",

    "password": "StrongPassword123!"

  }'

```



Health check:



```bash

curl http://localhost:8081/health

```



## Tests



```bash

go test ./...

```



Or from monorepo root:



```bash

make test-identity-service

```



## Docker



Build from monorepo root:



```bash

docker build -f services/identity-service/Dockerfile -t freight-platform/identity-service .

```



## Security notes



- Passwords are stored as bcrypt hashes only

- Passwords and JWT secrets are never logged

- Invalid login returns `401 UNAUTHORIZED`

- Non-active users receive `403 FORBIDDEN` on login


