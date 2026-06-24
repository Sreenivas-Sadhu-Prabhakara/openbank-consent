package consent

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sreeni/openbank-bian/pkg/obie"
)

// PgRepository is the Postgres-backed Repository. The consent service owns the
// "consent" schema; this type touches nothing outside it.
type PgRepository struct {
	pool *pgxpool.Pool
}

// NewPgRepository returns a Postgres repository over the given pool.
func NewPgRepository(pool *pgxpool.Pool) *PgRepository {
	return &PgRepository{pool: pool}
}

const consentColumns = `id, type, status, creation_dt, status_update_dt,
	permissions, expiration_dt, txn_from_dt, txn_to_dt,
	instruction_id, e2e_id, instructed_amount, instructed_currency,
	creditor_scheme, creditor_ident, creditor_name,
	debtor_scheme, debtor_ident, debtor_name, reference`

func (r *PgRepository) Create(ctx context.Context, c *Consent) error {
	var amount, currency *string
	if c.InstructedAmount != nil {
		a := c.InstructedAmount.String()
		cur := c.InstructedAmount.Currency
		amount, currency = &a, &cur
	}
	cScheme, cIdent, cName := accountCols(c.CreditorAccount)
	dScheme, dIdent, dName := accountCols(c.DebtorAccount)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO consent.consents (`+consentColumns+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)`,
		c.ID, string(c.Type), string(c.Status), c.CreationDateTime, c.StatusUpdateDateTime,
		c.Permissions, c.ExpirationDateTime, c.TransactionFromDateTime, c.TransactionToDateTime,
		nullable(c.InstructionIdentification), nullable(c.EndToEndIdentification), amount, currency,
		cScheme, cIdent, cName, dScheme, dIdent, dName, nullable(c.Reference),
	)
	return err
}

func (r *PgRepository) Get(ctx context.Context, id string) (*Consent, error) {
	row := r.pool.QueryRow(ctx, `SELECT `+consentColumns+` FROM consent.consents WHERE id = $1`, id)
	c, err := scanConsent(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return c, err
}

func (r *PgRepository) Update(ctx context.Context, c *Consent) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE consent.consents
		SET status = $2, status_update_dt = $3
		WHERE id = $1`,
		c.ID, string(c.Status), c.StatusUpdateDateTime,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *PgRepository) Delete(ctx context.Context, id string) error {
	tag, err := r.pool.Exec(ctx, `DELETE FROM consent.consents WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// scanConsent reads a row in consentColumns order into a Consent.
func scanConsent(row pgx.Row) (*Consent, error) {
	var (
		c                      Consent
		typ, status            string
		permissions            []string
		exp, from, to          *time.Time
		instrID, e2eID         *string
		amount, currency       *string
		cScheme, cIdent, cName *string
		dScheme, dIdent, dName *string
		reference              *string
	)
	if err := row.Scan(
		&c.ID, &typ, &status, &c.CreationDateTime, &c.StatusUpdateDateTime,
		&permissions, &exp, &from, &to,
		&instrID, &e2eID, &amount, &currency,
		&cScheme, &cIdent, &cName, &dScheme, &dIdent, &dName, &reference,
	); err != nil {
		return nil, err
	}

	c.Type = Type(typ)
	c.Status = Status(status)
	c.Permissions = permissions
	c.ExpirationDateTime = exp
	c.TransactionFromDateTime = from
	c.TransactionToDateTime = to
	c.InstructionIdentification = deref(instrID)
	c.EndToEndIdentification = deref(e2eID)
	c.Reference = deref(reference)

	if amount != nil && currency != nil {
		a, err := obie.NewAmount(*amount, *currency)
		if err != nil {
			return nil, err
		}
		c.InstructedAmount = &a
	}
	c.CreditorAccount = accountFromCols(cScheme, cIdent, cName)
	c.DebtorAccount = accountFromCols(dScheme, dIdent, dName)
	return &c, nil
}

func accountCols(a *Account) (scheme, ident, name *string) {
	if a == nil {
		return nil, nil, nil
	}
	return &a.SchemeName, &a.Identification, ptrOrNil(a.Name)
}

func accountFromCols(scheme, ident, name *string) *Account {
	if scheme == nil || ident == nil {
		return nil
	}
	return &Account{SchemeName: *scheme, Identification: *ident, Name: deref(name)}
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func ptrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
