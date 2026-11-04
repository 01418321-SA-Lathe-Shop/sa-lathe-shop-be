package postgres

import (
	"context"
	"errors"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var _ auth.CustomerRepository = (*Database)(nil)

func (d *Database) RegisterCustomer(ctx context.Context, in auth.CustomerRegistration) (auth.Account, error) {
	tx, err := d.identityBegin(ctx)
	if err != nil {
		return auth.Account{}, err
	}
	defer tx.Rollback(ctx)
	a := in.Account
	a.Role = auth.RoleCustomer
	// H-01 (UC1 Q1.E1): Check duplicate person by phone/email or duplicate company tax_id when supplied.
	if in.Phone != "" || in.Email != "" {
		var dup bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM person p WHERE ($1<>'' AND p.phone=$1) OR ($2<>'' AND lower(p.email)=lower($2)))`, in.Phone, in.Email).Scan(&dup); err != nil {
			return auth.Account{}, authError(err)
		}
		if dup {
			return auth.Account{}, auth.ErrConflict
		}
	}
	if in.TaxID != "" {
		var dupTax bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM customer c WHERE c.tax_id=$1)`, in.TaxID).Scan(&dupTax); err != nil {
			return auth.Account{}, authError(err)
		}
		if dupTax {
			return auth.Account{}, auth.ErrConflict
		}
	}
	var phonePtr, emailPtr *string
	if in.Phone != "" {
		phonePtr = &in.Phone
	}
	if in.Email != "" {
		emailPtr = &in.Email
	}
	_, err = tx.Exec(ctx, `INSERT INTO person(id,name,phone,email) VALUES($1,$2,$3,$4)`, a.PersonID, a.Name, phonePtr, emailPtr)
	if err != nil {
		return auth.Account{}, authError(err)
	}
	err = tx.QueryRow(ctx, `INSERT INTO account(id,person_id,username,password_hash,role) VALUES($1,$2,$3,$4,'customer') RETURNING created_at`, a.ID, a.PersonID, a.Username, a.PasswordHash).Scan(&a.CreatedAt)
	if err != nil {
		return auth.Account{}, authError(err)
	}
	if len(in.InvitationHash) > 0 {
		if err = acceptCompanyInvitation(ctx, tx, a.ID, in.InvitationHash); err != nil {
			return auth.Account{}, err
		}
	} else {
		name := a.Name
		if in.Type == "company" {
			name = in.CompanyName
		}
		// The sign-up address also fills the billing address used on printed documents.
		_, err = tx.Exec(ctx, `INSERT INTO customer(id,customer_type,name,billing_address,tax_id,billing_phone,billing_email) VALUES($1,$2,$3,$4,$5,$6,$7)`,
			in.CustomerID, in.Type, name, in.Address, in.TaxID, in.Phone, in.Email)
		if err != nil {
			return auth.Account{}, authError(err)
		}
		if in.Address != "" {
			_, err = tx.Exec(ctx, `INSERT INTO customer_address(id,customer_id,address,address_line1,address_line2,district,province,postal_code,is_default,created_by,contact_name) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
				uuid.NewString(), in.CustomerID, in.Address, in.AddressLine1, in.AddressLine2, in.District, in.Province, in.PostalCode, in.IsDefault, a.ID, in.ContactName)
			if err != nil {
				return auth.Account{}, authError(err)
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO customer_member(customer_id,account_id,is_owner) VALUES($1,$2,true)`, in.CustomerID, a.ID)
		if err != nil {
			return auth.Account{}, authError(err)
		}
	}
	return a, tx.Commit(ctx)
}
func (d *Database) CustomerProfiles(ctx context.Context, accountID string) ([]auth.CustomerProfile, error) {
	rows, err := d.identityQueries().Query(ctx, `SELECT c.id::text,c.customer_type,c.name,m.is_owner FROM customer c JOIN customer_member m ON m.customer_id=c.id JOIN account a ON a.id=m.account_id WHERE m.account_id=$1 AND a.is_active AND a.role='customer' ORDER BY c.created_at,c.id`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]auth.CustomerProfile, 0)
	for rows.Next() {
		var v auth.CustomerProfile
		if err = rows.Scan(&v.ID, &v.Type, &v.Name, &v.Owner); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (d *Database) SaveInvitation(ctx context.Context, in auth.Invitation) error {
	tag, err := d.identityQueries().Exec(ctx, `INSERT INTO customer_invitation(id,customer_id,created_by,token_hash,expires_at)
 SELECT $1,c.id,a.id,$4,$5 FROM customer c JOIN customer_member m ON m.customer_id=c.id JOIN account a ON a.id=m.account_id
 WHERE c.id=$2 AND a.id=$3 AND c.customer_type='company' AND m.is_owner AND a.is_active AND a.role='customer'`, in.ID, in.CustomerID, in.CreatedBy, in.Hash, in.ExpiresAt)
	if err != nil {
		return authError(err)
	}
	if tag.RowsAffected() != 1 {
		return auth.ErrForbidden
	}
	return nil
}
func acceptCompanyInvitation(ctx context.Context, tx pgx.Tx, accountID string, hash []byte) error {
	var invitationID, customerID string
	err := tx.QueryRow(ctx, `SELECT i.id::text,i.customer_id::text FROM customer_invitation i
 JOIN customer c ON c.id=i.customer_id JOIN account creator ON creator.id=i.created_by
 JOIN customer_member m ON m.customer_id=c.id AND m.account_id=creator.id
 WHERE i.token_hash=$1 AND i.accepted_at IS NULL AND i.expires_at>clock_timestamp()
 AND c.customer_type='company' AND creator.is_active AND m.is_owner FOR UPDATE OF i`, hash).Scan(&invitationID, &customerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrInvalidInput
	}
	if err != nil {
		return err
	}
	// Account activity is rechecked inside the membership transaction.
	var active bool
	err = tx.QueryRow(ctx, `SELECT is_active FROM account WHERE id=$1 AND role='customer' FOR SHARE`, accountID).Scan(&active)
	if err != nil {
		return authError(err)
	}
	if !active {
		return auth.ErrForbidden
	}
	_, err = tx.Exec(ctx, `INSERT INTO customer_member(customer_id,account_id) VALUES($1,$2)`, customerID, accountID)
	if err != nil {
		return authError(err)
	}
	_, err = tx.Exec(ctx, `UPDATE customer_invitation SET accepted_at=now() WHERE id=$1`, invitationID)
	return err
}
func (d *Database) AcceptInvitation(ctx context.Context, accountID string, hash []byte) error {
	tx, err := d.identityBegin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = acceptCompanyInvitation(ctx, tx, accountID, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

const activeCustomerMember = `EXISTS(SELECT 1 FROM customer_member m JOIN account a ON a.id=m.account_id WHERE m.customer_id=$2 AND m.account_id=$1 AND a.is_active AND a.role='customer'`

func (d *Database) CustomerAddresses(ctx context.Context, accountID, customerID string) ([]auth.CustomerAddress, error) {
	var member bool
	if err := d.identityQueries().QueryRow(ctx, `SELECT `+activeCustomerMember+`)`, accountID, customerID).Scan(&member); err != nil {
		return nil, err
	}
	if !member {
		return nil, auth.ErrForbidden
	}
	rows, err := d.identityQueries().Query(ctx, `SELECT id::text,contact_name,address,address_line1,address_line2,district,province,postal_code,is_default,created_at FROM customer_address WHERE customer_id=$1 ORDER BY is_default DESC,created_at,id`, customerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]auth.CustomerAddress, 0)
	for rows.Next() {
		var v auth.CustomerAddress
		if err = rows.Scan(&v.ID, &v.ContactName, &v.Address, &v.AddressLine1, &v.AddressLine2, &v.District, &v.Province, &v.PostalCode, &v.IsDefault, &v.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (d *Database) AddCustomerAddress(ctx context.Context, accountID, customerID string, in auth.CustomerAddress) (auth.CustomerAddress, error) {
	return d.saveCustomerAddress(ctx, accountID, customerID, in, `INSERT INTO customer_address(id,customer_id,address,address_line1,address_line2,district,province,postal_code,is_default,created_by,contact_name)
 VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) RETURNING created_at`)
}

// Only the owner's own customer rows match; another customer's address id reads as not found.
func (d *Database) UpdateCustomerAddress(ctx context.Context, accountID, customerID string, in auth.CustomerAddress) (auth.CustomerAddress, error) {
	return d.saveCustomerAddress(ctx, accountID, customerID, in, `UPDATE customer_address SET address=$3,address_line1=$4,address_line2=$5,district=$6,province=$7,postal_code=$8,is_default=$9,contact_name=$11
 WHERE id=$1 AND customer_id=$2 AND $10::uuid IS NOT NULL RETURNING created_at`) // $10 (creator) is unused here but typed so both queries share one argument list.
}
func (d *Database) saveCustomerAddress(ctx context.Context, accountID, customerID string, in auth.CustomerAddress, query string) (auth.CustomerAddress, error) {
	tx, err := d.identityBegin(ctx)
	if err != nil {
		return auth.CustomerAddress{}, err
	}
	defer tx.Rollback(ctx)
	var owner bool
	if err = tx.QueryRow(ctx, `SELECT `+activeCustomerMember+` AND m.is_owner)`, accountID, customerID).Scan(&owner); err != nil {
		return auth.CustomerAddress{}, err
	}
	if !owner {
		return auth.CustomerAddress{}, auth.ErrForbidden
	}
	if in.IsDefault {
		if _, err = tx.Exec(ctx, `UPDATE customer_address SET is_default=false WHERE customer_id=$1`, customerID); err != nil {
			return auth.CustomerAddress{}, authError(err)
		}
		if _, err = tx.Exec(ctx, `UPDATE customer SET billing_address=$2 WHERE id=$1`, customerID, in.Address); err != nil {
			return auth.CustomerAddress{}, authError(err)
		}
	}
	err = tx.QueryRow(ctx, query, in.ID, customerID, in.Address, in.AddressLine1, in.AddressLine2, in.District, in.Province, in.PostalCode, in.IsDefault, accountID, in.ContactName).Scan(&in.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.CustomerAddress{}, auth.ErrNotFound
	}
	if err != nil {
		return auth.CustomerAddress{}, authError(err)
	}
	return in, tx.Commit(ctx)
}
