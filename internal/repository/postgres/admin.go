package postgres

import (
	"context"

	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/jackc/pgx/v5"
)

const adminSelect = `SELECT ` + accountFields + `,p.email,p.phone FROM account a JOIN person p ON p.id=a.person_id JOIN admin ad ON ad.person_id=p.id WHERE a.role='admin'`

func adminRow(row pgx.Row) (auth.Admin, error) {
	var v auth.Admin
	err := row.Scan(append(accountTargets(&v.Account), &v.Email, &v.Phone)...)
	return v, authError(err)
}
func (d *Database) ListAdmins(ctx context.Context) ([]auth.Admin, error) {
	rows, err := d.pool.Query(ctx, adminSelect+` ORDER BY a.created_at,a.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]auth.Admin, 0)
	for rows.Next() {
		v, err := adminRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func (d *Database) CreateAdmin(ctx context.Context, input auth.NewStaff) (auth.Admin, error) {
	tx, err := d.identityBegin(ctx)
	if err != nil {
		return auth.Admin{}, err
	}
	defer tx.Rollback(ctx)
	input.Account.Role = auth.RoleAdmin
	input.Account.SuperAdmin = false
	a, err := insertStaff(ctx, tx, input)
	if err != nil {
		return auth.Admin{}, err
	}
	return auth.Admin{Account: a, Email: &input.Email, Phone: &input.Phone}, tx.Commit(ctx)
}
func (d *Database) SetAdminActive(ctx context.Context, id string, active bool) (auth.Admin, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return auth.Admin{}, err
	}
	defer tx.Rollback(ctx)
	v, err := adminRow(tx.QueryRow(ctx, adminSelect+` AND a.id=$1 FOR UPDATE OF a`, id))
	if err != nil {
		return auth.Admin{}, err
	}
	if v.SuperAdmin {
		return auth.Admin{}, auth.ErrForbidden
	}
	if _, err = tx.Exec(ctx, `UPDATE account SET is_active=$2,edited_at=now() WHERE id=$1`, id, active); err != nil {
		return auth.Admin{}, authError(err)
	}
	if !active {
		if _, err = tx.Exec(ctx, `UPDATE auth_session SET revoked_at=now() WHERE account_id=$1 AND revoked_at IS NULL`, id); err != nil {
			return auth.Admin{}, err
		}
	}
	v.Active = active
	return v, tx.Commit(ctx)
}
