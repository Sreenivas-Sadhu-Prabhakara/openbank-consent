-- Consent service schema. Owned exclusively by the consent microservice;
-- no other service reads or writes these tables.
CREATE SCHEMA IF NOT EXISTS consent;

CREATE TABLE IF NOT EXISTS consent.consents (
    id                  TEXT PRIMARY KEY,
    type                TEXT        NOT NULL,
    status              TEXT        NOT NULL,
    creation_dt         TIMESTAMPTZ NOT NULL,
    status_update_dt    TIMESTAMPTZ NOT NULL,

    -- account-access (AIS)
    permissions         TEXT[],
    expiration_dt       TIMESTAMPTZ,
    txn_from_dt         TIMESTAMPTZ,
    txn_to_dt           TIMESTAMPTZ,

    -- domestic-payment (PIS)
    instruction_id      TEXT,
    e2e_id              TEXT,
    instructed_amount   TEXT,
    instructed_currency TEXT,
    creditor_scheme     TEXT,
    creditor_ident      TEXT,
    creditor_name       TEXT,
    reference           TEXT,

    -- shared debtor account (PIS + CBPII)
    debtor_scheme       TEXT,
    debtor_ident        TEXT,
    debtor_name         TEXT
);

CREATE INDEX IF NOT EXISTS idx_consents_type_status ON consent.consents (type, status);
