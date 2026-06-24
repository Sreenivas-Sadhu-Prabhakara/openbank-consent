# Changelog

All notable changes to **openbank-consent** are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and this project
adheres to [Semantic Versioning](https://semver.org/).

## [1.0.0] - 2026-06-24

Initial release of the Consent microservice (BIAN Consent Administration / OBIE consent lifecycle).

### Added

- Account-access, domestic-payment and funds-confirmation consent endpoints (OBIE).
- Consent lifecycle modelled in code: authorise, validate, consume, revoke, with expiry.
- Internal API for service-to-service consent validation.
- In-memory and Postgres repository adapters; SQL migrations applied on startup.
- OBIE-shaped HTTP API with FAPI interaction-id, structured logging and panic recovery.
- Unit/handler test suite plus Postgres integration tests (testcontainers).
- GitHub Actions CI and MIT license.
