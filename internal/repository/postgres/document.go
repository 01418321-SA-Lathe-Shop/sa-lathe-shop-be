package postgres

import (
	"context"
	"errors"

	doc "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/document"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ doc.Repository = (*Database)(nil)

func docError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return doc.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return doc.ErrConflict
		case "23514", "22001", "22007", "22008", "22P02":
			return doc.ErrInvalidInput
		}
	}
	return err
}

const shopFields = `name,address,tax_id,phone,email,bank_name,bank_account_name,bank_account_no,lock_version,updated_at`

func shopRow(row pgx.Row) (doc.ShopProfile, error) {
	var v doc.ShopProfile
	err := row.Scan(&v.Name, &v.Address, &v.TaxID, &v.Phone, &v.Email, &v.BankName, &v.BankAccountName, &v.BankAccountNo, &v.LockVersion, &v.UpdatedAt)
	return v, docError(err)
}
func (d *Database) GetShopProfile(ctx context.Context) (doc.ShopProfile, error) {
	v, err := shopRow(d.pool.QueryRow(ctx, `SELECT `+shopFields+` FROM shop_profile`))
	if errors.Is(err, doc.ErrNotFound) {
		return doc.ShopProfile{}, nil
	}
	return v, err
}
func (d *Database) SaveShopProfile(ctx context.Context, actor string, in doc.ShopInput) (doc.ShopProfile, error) {
	p := in.ShopProfile
	query := `INSERT INTO shop_profile(id,name,address,tax_id,phone,email,bank_name,bank_account_name,bank_account_no,updated_by)
 SELECT true,$3,$4,$5,$6,$7,$8,$9,$10,$1 WHERE $2=0 AND EXISTS(SELECT 1 FROM account a WHERE a.id=$1 AND a.is_active AND a.role='admin')
 ON CONFLICT(id) DO NOTHING RETURNING ` + shopFields
	if in.ExpectedLockVersion != 0 {
		query = `UPDATE shop_profile SET name=$3,address=$4,tax_id=$5,phone=$6,email=$7,bank_name=$8,bank_account_name=$9,bank_account_no=$10,
 lock_version=lock_version+1,updated_by=$1,updated_at=now()
 WHERE lock_version=$2 AND EXISTS(SELECT 1 FROM account a WHERE a.id=$1 AND a.is_active AND a.role='admin') RETURNING ` + shopFields
	}
	v, err := shopRow(d.pool.QueryRow(ctx, query,
		actor, in.ExpectedLockVersion, p.Name, p.Address, p.TaxID, p.Phone, p.Email, p.BankName, p.BankAccountName, p.BankAccountNo))
	if errors.Is(err, doc.ErrNotFound) {
		return v, doc.ErrConflict
	}
	return v, err
}

const billingFields = `c.id::text,c.name,c.customer_type,c.billing_address,c.tax_id,c.billing_phone,c.billing_email,c.billing_lock_version`
const customerMember = `EXISTS(SELECT 1 FROM customer_member m JOIN account a ON a.id=m.account_id WHERE m.account_id=$1 AND m.customer_id=c.id AND a.is_active AND a.role='customer'`

func billingRow(row pgx.Row) (doc.Billing, error) {
	var v doc.Billing
	err := row.Scan(&v.CustomerID, &v.Name, &v.Type, &v.Address, &v.TaxID, &v.Phone, &v.Email, &v.LockVersion)
	return v, docError(err)
}
func (d *Database) GetBilling(ctx context.Context, actor, customerID string) (doc.Billing, error) {
	return billingRow(d.pool.QueryRow(ctx, `SELECT `+billingFields+` FROM customer c WHERE c.id=$2 AND (`+customerMember+`)
 OR EXISTS(SELECT 1 FROM account a WHERE a.id=$1 AND a.is_active AND a.role IN ('admin','worker')))`, actor, customerID))
}
func (d *Database) SaveBilling(ctx context.Context, actor, customerID string, in doc.BillingInput) (doc.Billing, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return doc.Billing{}, err
	}
	defer tx.Rollback(ctx)
	var owner bool
	var version int64
	err = tx.QueryRow(ctx, `SELECT `+customerMember+` AND m.is_owner),c.billing_lock_version FROM customer c WHERE c.id=$2 AND `+customerMember+`) FOR UPDATE OF c`, actor, customerID).Scan(&owner, &version)
	if err != nil {
		return doc.Billing{}, docError(err)
	}
	if !owner {
		return doc.Billing{}, doc.ErrForbidden
	}
	if version != in.ExpectedLockVersion {
		return doc.Billing{}, doc.ErrConflict
	}
	v, err := billingRow(tx.QueryRow(ctx, `UPDATE customer c SET billing_address=$2,tax_id=$3,billing_phone=$4,billing_email=$5,billing_lock_version=billing_lock_version+1 WHERE c.id=$1 RETURNING `+billingFields,
		customerID, in.Address, in.TaxID, in.Phone, in.Email))
	if err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}

const poFields = `p.quotation_id::text,p.po_no,p.issued_on::text,p.remarks,p.lock_version,p.updated_at`

func poRow(row pgx.Row) (doc.CustomerPO, error) {
	var v doc.CustomerPO
	err := row.Scan(&v.QuotationID, &v.PONo, &v.IssuedOn, &v.Remarks, &v.LockVersion, &v.UpdatedAt)
	return v, docError(err)
}
func (d *Database) GetCustomerPO(ctx context.Context, actor, quotationID string) (doc.CustomerPO, error) {
	return poRow(d.pool.QueryRow(ctx, `SELECT `+poFields+` FROM customer_po p JOIN quotation q ON q.id=p.quotation_id JOIN job_request r ON r.id=q.request_id WHERE p.quotation_id=$2 AND `+accessibleRequest, actor, quotationID))
}
func (d *Database) SaveCustomerPO(ctx context.Context, actor, quotationID string, in doc.POInput) (doc.CustomerPO, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return doc.CustomerPO{}, err
	}
	defer tx.Rollback(ctx)
	// Only a member of the requesting customer may write, and only while a PO is still needed.
	var required, approved bool
	err = tx.QueryRow(ctx, `SELECT t.po_required,EXISTS(SELECT 1 FROM term_evidence e WHERE e.quotation_id=t.quotation_id AND e.kind='po' AND e.status='approved')
 FROM quotation_acceptance_terms t JOIN quotation q ON q.id=t.quotation_id JOIN job_request r ON r.id=q.request_id JOIN customer c ON c.id=r.customer_id
 WHERE t.quotation_id=$2 AND `+customerMember+`) FOR UPDATE OF t`, actor, quotationID).Scan(&required, &approved)
	if err != nil {
		return doc.CustomerPO{}, docError(err)
	}
	if !required || approved {
		return doc.CustomerPO{}, doc.ErrConflict
	}
	var row pgx.Row
	if in.ExpectedLockVersion == 0 {
		row = tx.QueryRow(ctx, `INSERT INTO customer_po AS p(quotation_id,po_no,issued_on,remarks,created_by,updated_by) VALUES($1,$2,$3::date,$4,$5,$5) RETURNING `+poFields, quotationID, in.PONo, in.IssuedOn, in.Remarks, actor)
	} else {
		row = tx.QueryRow(ctx, `UPDATE customer_po p SET po_no=$2,issued_on=$3::date,remarks=$4,updated_by=$5,updated_at=now(),lock_version=lock_version+1 WHERE p.quotation_id=$1 AND p.lock_version=$6 RETURNING `+poFields, quotationID, in.PONo, in.IssuedOn, in.Remarks, actor, in.ExpectedLockVersion)
	}
	v, err := poRow(row)
	if errors.Is(err, doc.ErrNotFound) {
		return v, doc.ErrConflict
	}
	if err != nil {
		return v, err
	}
	return v, tx.Commit(ctx)
}
