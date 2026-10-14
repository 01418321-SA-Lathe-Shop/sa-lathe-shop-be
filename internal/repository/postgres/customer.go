package postgres

import (
	"context"
	"errors"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/jackc/pgx/v5"
)

var _ auth.CustomerRepository = (*Database)(nil)

func (d *Database) RegisterCustomer(ctx context.Context, in auth.CustomerRegistration) (auth.Account, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return auth.Account{}, err
	}
	defer tx.Rollback(ctx)
	a := in.Account
	a.Role = auth.RoleCustomer
	_, err = tx.Exec(ctx, `INSERT INTO person(id,name) VALUES($1,$2)`, a.PersonID, a.Name)
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
		_, err = tx.Exec(ctx, `INSERT INTO customer(id,customer_type,name) VALUES($1,$2,$3)`, in.CustomerID, in.Type, name)
		if err != nil {
			return auth.Account{}, authError(err)
		}
		_, err = tx.Exec(ctx, `INSERT INTO customer_member(customer_id,account_id,is_owner) VALUES($1,$2,true)`, in.CustomerID, a.ID)
		if err != nil {
			return auth.Account{}, authError(err)
		}
	}
	return a, tx.Commit(ctx)
}
func (d *Database) CustomerProfiles(ctx context.Context, accountID string) ([]auth.CustomerProfile, error) {
	rows, err := d.pool.Query(ctx, `SELECT c.id::text,c.customer_type,c.name,m.is_owner FROM customer c JOIN customer_member m ON m.customer_id=c.id JOIN account a ON a.id=m.account_id WHERE m.account_id=$1 AND a.is_active AND a.role='customer' ORDER BY c.created_at,c.id`, accountID)
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
	tag, err := d.pool.Exec(ctx, `INSERT INTO customer_invitation(id,customer_id,created_by,token_hash,expires_at)
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
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err = acceptCompanyInvitation(ctx, tx, accountID, hash); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
