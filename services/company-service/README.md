# company-service



Manages companies and company memberships within a tenant in the freight platform (`core.companies`, `core.company_memberships`).



## Responsibilities



- Create, read, list, update companies

- Soft delete companies (`deleted_at`, `status = DELETED`)

- Add users to companies via memberships

- List company members with roles

- Update and soft delete memberships

- Assign roles to members on creation (`core.user_roles`)



## Environment variables



| Variable | Default | Description |

|----------|---------|-------------|

| `COMPANY_SERVICE_PORT` | `8082` | HTTP port |

| `DATABASE_URL` | local postgres URL | PostgreSQL connection string |

| `LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |

| `ENVIRONMENT` | `development` | Runtime environment |



## Run locally



From monorepo root:



```bash

make run-company-service

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

| POST | `/v1/companies` | Create company |

| GET | `/v1/companies/{id}` | Get company by ID |

| GET | `/v1/companies` | List companies |

| PATCH | `/v1/companies/{id}` | Update company |

| DELETE | `/v1/companies/{id}` | Soft delete company |

| POST | `/v1/companies/{company_id}/members` | Add user to company |

| GET | `/v1/companies/{company_id}/members` | List company members |

| PATCH | `/v1/companies/{company_id}/members/{membership_id}` | Update membership |

| DELETE | `/v1/companies/{company_id}/members/{membership_id}` | Soft delete membership |



## Examples



### 1. Create a tenant (one-time for local dev)



```bash

docker exec -i freight_postgres psql -U freight -d freight_platform -c \

  "INSERT INTO core.tenants (code, name) VALUES ('demo', 'Demo Tenant') ON CONFLICT (code) DO NOTHING RETURNING id;"

```



Save the returned `id` as `TENANT_ID`.



### 2. Create a company



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



### 3. Create a user (identity-service on port 8081)



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



Get a role id (e.g. `SHIPPER_LOGIST`):



```bash

curl "http://localhost:8081/v1/roles?tenant_id=TENANT_ID"

```



Save a role `id` as `ROLE_ID`.



### 4. Add user to company



```bash

curl -X POST http://localhost:8082/v1/companies/COMPANY_ID/members \

  -H "Content-Type: application/json" \

  -d '{

    "tenant_id": "TENANT_ID",

    "user_id": "USER_ID",

    "position": "Логист",

    "role_id": "ROLE_ID"

  }'

```



### 5. List company members



```bash

curl "http://localhost:8082/v1/companies/COMPANY_ID/members?tenant_id=TENANT_ID&status=ACTIVE&limit=20&offset=0"

```



### 6. Get user companies (identity-service)



```bash

curl "http://localhost:8081/v1/users/USER_ID/companies?tenant_id=TENANT_ID&status=ACTIVE"

```



### 7. Assign role to user in company (identity-service)



```bash

curl -X POST http://localhost:8081/v1/users/USER_ID/companies/COMPANY_ID/roles \

  -H "Content-Type: application/json" \

  -d '{

    "tenant_id": "TENANT_ID",

    "role_id": "ROLE_ID"

  }'

```



Health check:



```bash

curl http://localhost:8082/health

```



## Error format



```json

{

  "error": {

    "code": "VALIDATION_ERROR",

    "message": "tenant_id is required",

    "details": {}

  }

}

```



## Tests



```bash

go test ./...

```



Or from monorepo root:



```bash

make test-company-service

```



## Docker



Build from monorepo root:



```bash

docker build -f services/company-service/Dockerfile -t freight-platform/company-service .

```


