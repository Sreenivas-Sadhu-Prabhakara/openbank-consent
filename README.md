# openbank-consent

The **Consent** microservice — the BIAN *Consent Administration* service domain. It owns the full UK Open Banking (OBIE) consent lifecycle for all three consent types in the estate: **account-access** (AIS), **domestic-payment** (PIS) and **funds-confirmation** (CBPII). The AIS/PIS/CBPII services never store consent themselves — they validate against this service.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| POST | `/account-access-consents` | Create an AIS consent (`AwaitingAuthorisation`) |
| GET | `/account-access-consents/{consentId}` | Read an AIS consent |
| DELETE | `/account-access-consents/{consentId}` | Revoke/remove an AIS consent |
| POST | `/domestic-payment-consents` | Create a PIS consent |
| GET | `/domestic-payment-consents/{consentId}` | Read a PIS consent |
| POST | `/funds-confirmation-consents` | Create a CBPII consent |
| GET | `/funds-confirmation-consents/{consentId}` | Read a CBPII consent |
| DELETE | `/funds-confirmation-consents/{consentId}` | Remove a CBPII consent |
| GET | `/internal/consents/{consentId}` | Service-to-service consent projection |
| POST | `/internal/consents/{consentId}/authorise` | Stand-in for PSU authentication → `Authorised` |
| POST | `/internal/consents/{consentId}/consume` | Mark single-use payment consent `Consumed` |
| GET | `/health` | Liveness |

Lifecycle: `AwaitingAuthorisation → Authorised → Consumed` (payments), with `Rejected`/`Revoked` terminal states.

## Configuration

| Env | Default | Notes |
|---|---|---|
| `ADDR` | `:8081` | Listen address |
| `BASE_URL` | `http://localhost:8081` | Used for `Links.Self` |
| `DATABASE_URL` | _(unset)_ | Postgres DSN; **unset → in-memory store** (zero infra) |

## Run

```bash
go run .                                   # in-memory, no infrastructure
DATABASE_URL=postgres://... go run .       # Postgres (migrations applied on boot)
docker build -t openbank/consent . && docker run -p 8081:8081 openbank/consent
```

## Test

```bash
go test ./...                       # unit + handler tests (no Docker)
go test -tags=integration ./...     # Postgres repo tests via testcontainers (needs Docker)
```

## Layout notes

- `internal/consent/` — domain, `Repository` port (in-memory + Postgres adapters), service logic, OBIE HTTP handlers.
- `migrations/` — SQL owned by this service; applied automatically on startup when `DATABASE_URL` is set.
- `pkg/` — vendored copy of the shared OBIE library, wired via `replace github.com/sreeni/openbank-bian/pkg => ./pkg`, so this repo builds standalone.
