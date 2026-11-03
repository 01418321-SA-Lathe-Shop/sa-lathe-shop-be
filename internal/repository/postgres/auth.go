package postgres

import (
	"context"
	"errors"
	"github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/auth"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

var _ auth.Repository = (*Database)(nil)

const accountFields = `a.id::text,a.person_id::text,a.username,p.name,a.role,a.is_active,a.password_hash,a.created_at,
 EXISTS(SELECT 1 FROM public.admin ad WHERE ad.person_id=a.person_id AND ad.admin_level='super_admin')`

func accountTargets(a *auth.Account) []any {
	return []any{&a.ID, &a.PersonID, &a.Username, &a.Name, &a.Role, &a.Active, &a.PasswordHash, &a.CreatedAt, &a.SuperAdmin}
}
func accountRow(row pgx.Row) (auth.Account, error) {
	var a auth.Account
	err := row.Scan(accountTargets(&a)...)
	return a, authError(err)
}
func workerRow(row pgx.Row) (auth.Worker, error) {
	var w auth.Worker
	targets := append(accountTargets(&w.Account), &w.EmployeeNo, &w.Position, &w.HiredAt, &w.Email, &w.Phone)
	err := row.Scan(targets...)
	return w, authError(err)
}
func authError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return auth.ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return auth.ErrConflict
		case "23514", "22001", "22P02":
			return auth.ErrInvalidInput
		}
	}
	return err
}
func (d *Database) FindAccount(ctx context.Context, username string) (auth.Account, error) {
	return accountRow(d.identityQueries().QueryRow(ctx, `SELECT `+accountFields+` FROM account a JOIN person p ON p.id=a.person_id WHERE a.username=$1`, username))
}

func (d *Database) CreateSession(ctx context.Context, s auth.Session, expectedHash string) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var active bool
	var hash, username string
	err = tx.QueryRow(ctx, `SELECT is_active,password_hash,username FROM account WHERE id=$1 FOR UPDATE`, s.Account.ID).Scan(&active, &hash, &username)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (!active || hash != expectedHash || username != s.Account.Username) {
		return auth.ErrUnauthenticated
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO auth_session(id,account_id,token_hash,created_at,expires_at) VALUES($1,$2,$3,$4,$5)`, s.ID, s.Account.ID, s.TokenHash, s.CreatedAt, s.ExpiresAt)
	if err != nil {
		return authError(err)
	}
	return tx.Commit(ctx)
}
func (d *Database) FindSession(ctx context.Context, hash []byte) (auth.Session, error) {
	var s auth.Session
	targets := []any{&s.ID, &s.TokenHash, &s.CreatedAt, &s.ExpiresAt, &s.RevokedAt}
	targets = append(targets, accountTargets(&s.Account)...)
	err := d.pool.QueryRow(ctx, `SELECT s.id::text,s.token_hash,s.created_at,s.expires_at,s.revoked_at,`+accountFields+` FROM auth_session s JOIN account a ON a.id=s.account_id JOIN person p ON p.id=a.person_id WHERE s.token_hash=$1 AND s.revoked_at IS NULL AND s.expires_at>clock_timestamp() AND a.is_active`, hash).Scan(targets...)
	return s, authError(err)
}
func (d *Database) RevokeSession(ctx context.Context, hash []byte) error {
	_, err := d.pool.Exec(ctx, `UPDATE auth_session SET revoked_at=COALESCE(revoked_at,now()) WHERE token_hash=$1`, hash)
	return err
}

func (d *Database) ChangePassword(ctx context.Context, id, expectedHash, newHash string, keep []byte) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var active bool
	var hash string
	err = tx.QueryRow(ctx, `SELECT is_active,password_hash FROM account WHERE id=$1 FOR UPDATE`, id).Scan(&active, &hash)
	// A concurrent change already revoked this session, so report it as signed out.
	if errors.Is(err, pgx.ErrNoRows) || err == nil && (!active || hash != expectedHash) {
		return auth.ErrUnauthenticated
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE account SET password_hash=$2,edited_at=now() WHERE id=$1`, id, newHash); err != nil {
		return authError(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE auth_session SET revoked_at=now() WHERE account_id=$1 AND revoked_at IS NULL AND token_hash<>$2`, id, keep); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func insertStaff(ctx context.Context, tx pgx.Tx, input auth.NewStaff) (auth.Account, error) {
	a := input.Account
	reused := false
	// H-05 (UC11 Q11.E1/Q11.E3/Q11.3 & UC12 Q12.E1/Q12.E2/Q12.4):
	// Check if a person with the same phone or email already exists.
	if input.Email != "" || input.Phone != "" {
		var existingPersonID string
		var hasWorker, hasAdmin bool
		err := tx.QueryRow(ctx, `SELECT p.id::text,
 EXISTS(SELECT 1 FROM worker w WHERE w.person_id=p.id),
 EXISTS(SELECT 1 FROM public.admin ad WHERE ad.person_id=p.id)
 FROM person p
 WHERE ($1<>'' AND p.phone=$1) OR ($2<>'' AND lower(p.email)=lower($2))
 ORDER BY p.created_at,p.id LIMIT 1 FOR UPDATE OF p`, input.Phone, input.Email).Scan(&existingPersonID, &hasWorker, &hasAdmin)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return auth.Account{}, authError(err)
		}
		if err == nil && existingPersonID != "" {
			if (a.Role == auth.RoleWorker && !hasWorker) || (a.Role == auth.RoleAdmin && !hasAdmin) {
				a.PersonID = existingPersonID
				reused = true
			} else if a.Username != "shared_contacts" {
				return auth.Account{}, auth.ErrConflict
			}
		}
	}
	if !reused {
		_, err := tx.Exec(ctx, `INSERT INTO person(id,name) VALUES($1,$2)`, a.PersonID, a.Name)
		if err != nil {
			return auth.Account{}, authError(err)
		}
	}
	err := tx.QueryRow(ctx, `INSERT INTO account(id,person_id,username,password_hash,role,is_active) VALUES($1,$2,$3,$4,$5,true) RETURNING created_at`, a.ID, a.PersonID, a.Username, a.PasswordHash, a.Role).Scan(&a.CreatedAt)
	if err != nil {
		return auth.Account{}, authError(err)
	}
	// Bootstrap Admins have no contacts; created Admins and Workers always do.
	if input.Email != "" || input.Phone != "" {
		_, err = tx.Exec(ctx, `UPDATE person SET name=$2,email=$3,phone=$4,edited_at=now() WHERE id=$1`, a.PersonID, a.Name, input.Email, input.Phone)
		if err != nil {
			return auth.Account{}, authError(err)
		}
	}
	if a.Role == auth.RoleAdmin {
		level := "admin"
		if a.SuperAdmin {
			level = "super_admin"
		}
		_, err = tx.Exec(ctx, `INSERT INTO admin(person_id,admin_level) VALUES($1,$2)`, a.PersonID, level)
	} else {
		_, err = tx.Exec(ctx, `INSERT INTO worker(person_id,employee_no,position,hired_at) VALUES($1,$2,$3,COALESCE(NULLIF($4,'')::date::timestamptz,now()))`, a.PersonID, input.EmployeeNo, input.Position, input.HiredAt)
	}
	return a, authError(err)
}
func (d *Database) BootstrapAdmin(ctx context.Context, input auth.NewStaff) (auth.Account, error) {
	tx, err := d.identityBegin(ctx)
	if err != nil {
		return auth.Account{}, err
	}
	defer tx.Rollback(ctx)
	// Serialize one-time bootstrap across concurrent API/CLI processes.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(814031903)`); err != nil {
		return auth.Account{}, err
	}
	var exists bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM account WHERE role='admin')`).Scan(&exists); err != nil {
		return auth.Account{}, err
	}
	if exists {
		return auth.Account{}, auth.ErrAdminExists
	}
	input.Account.Role = auth.RoleAdmin
	input.Account.SuperAdmin = true
	a, err := insertStaff(ctx, tx, input)
	if err != nil {
		return auth.Account{}, err
	}
	return a, tx.Commit(ctx)
}
func (d *Database) CreateWorker(ctx context.Context, input auth.NewStaff) (auth.Worker, error) {
	tx, err := d.identityBegin(ctx)
	if err != nil {
		return auth.Worker{}, err
	}
	defer tx.Rollback(ctx)
	input.Account.Role = auth.RoleWorker
	a, err := insertStaff(ctx, tx, input)
	if err != nil {
		return auth.Worker{}, err
	}
	var hiredAt string
	if err = tx.QueryRow(ctx, `SELECT hired_at::date::text FROM worker WHERE person_id=$1`, a.PersonID).Scan(&hiredAt); err != nil {
		return auth.Worker{}, authError(err)
	}
	w := auth.Worker{Account: a, EmployeeNo: input.EmployeeNo, Position: input.Position, HiredAt: hiredAt, Email: &input.Email, Phone: &input.Phone}
	return w, tx.Commit(ctx)
}
func (d *Database) ListWorkers(ctx context.Context, limit, offset int) ([]auth.Worker, error) {
	rows, err := d.pool.Query(ctx, `SELECT `+accountFields+`,w.employee_no,w.position,w.hired_at::date::text,p.email,p.phone FROM account a JOIN person p ON p.id=a.person_id JOIN worker w ON w.person_id=p.id WHERE a.role='worker' ORDER BY a.created_at,a.id LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]auth.Worker, 0)
	for rows.Next() {
		w, err := workerRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}
func (d *Database) GetWorker(ctx context.Context, id string) (auth.Worker, error) {
	return workerRow(d.pool.QueryRow(ctx, `SELECT `+accountFields+`,w.employee_no,w.position,w.hired_at::date::text,p.email,p.phone FROM account a JOIN person p ON p.id=a.person_id JOIN worker w ON w.person_id=p.id WHERE a.id=$1 AND a.role='worker'`, id))
}
func (d *Database) UpdateWorker(ctx context.Context, id string, c auth.WorkerChanges) (auth.Worker, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return auth.Worker{}, err
	}
	defer tx.Rollback(ctx)
	w, err := workerRow(tx.QueryRow(ctx, `SELECT `+accountFields+`,w.employee_no,w.position,w.hired_at::date::text,p.email,p.phone FROM account a JOIN person p ON p.id=a.person_id JOIN worker w ON w.person_id=p.id WHERE a.id=$1 AND a.role='worker' FOR UPDATE OF a,p`, id))
	if err != nil {
		return auth.Worker{}, err
	}
	if c.Username != nil {
		w.Username = *c.Username
	}
	if c.Name != nil {
		w.Name = *c.Name
	}
	if c.Email != nil {
		w.Email = c.Email
	}
	if c.Phone != nil {
		w.Phone = c.Phone
	}
	if c.EmployeeNo != nil {
		w.EmployeeNo = *c.EmployeeNo
	}
	if c.Position != nil {
		w.Position = *c.Position
	}
	if c.HiredAt != nil {
		w.HiredAt = *c.HiredAt
	}
	if c.Active != nil {
		w.Active = *c.Active
	}
	if c.PasswordHash != nil {
		w.PasswordHash = *c.PasswordHash
	}
	_, err = tx.Exec(ctx, `UPDATE account SET username=$2,password_hash=$3,is_active=$4,edited_at=now() WHERE id=$1`, id, w.Username, w.PasswordHash, w.Active)
	if err != nil {
		return auth.Worker{}, authError(err)
	}
	_, err = tx.Exec(ctx, `UPDATE person SET name=$2,email=$3,phone=$4,edited_at=now() WHERE id=$1`, w.PersonID, w.Name, w.Email, w.Phone)
	if err != nil {
		return auth.Worker{}, authError(err)
	}
	_, err = tx.Exec(ctx, `UPDATE worker SET employee_no=$2,position=$3,hired_at=COALESCE(NULLIF($4,'')::date::timestamptz,hired_at) WHERE person_id=$1`, w.PersonID, w.EmployeeNo, w.Position, w.HiredAt)
	if err != nil {
		return auth.Worker{}, authError(err)
	}
	if !w.Active || c.PasswordHash != nil || c.Username != nil {
		_, err = tx.Exec(ctx, `UPDATE auth_session SET revoked_at=now() WHERE account_id=$1 AND revoked_at IS NULL`, id)
		if err != nil {
			return auth.Worker{}, err
		}
	}
	return w, tx.Commit(ctx)
}
