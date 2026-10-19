package postgres

import (
	"context"
	"encoding/json"
	"errors"
	m "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/material"
	req "github.com/Yoshi4489/SA-Lathe-Shop-BE/internal/request"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"sort"
)

func loadPlan(ctx context.Context, tx pgx.Tx, jobID string) (m.Plan, error) {
	value := m.Plan{JobID: jobID, Lines: []m.PlanLine{}}
	var jobStatus string
	if err := tx.QueryRow(ctx, `SELECT material_plan_version,status FROM job WHERE id=$1`, jobID).Scan(&value.Version, &jobStatus); err != nil {
		return value, inventoryError(err)
	}
	rows, err := tx.Query(ctx, `SELECT r.material_id::text,r.required_qty::text,COALESCE(s.qty_reserved,0)::text,COALESCE(s.qty_consumed,0)::text FROM job_material_requirement r LEFT JOIN material_reservation s USING(job_id,material_id) WHERE r.job_id=$1 AND r.required_qty>0 ORDER BY r.material_id`, jobID)
	if err != nil {
		return value, err
	}
	ready := true
	for rows.Next() {
		var line m.PlanLine
		var required, reserved, consumed string
		if err = rows.Scan(&line.MaterialID, &required, &reserved, &consumed); err != nil {
			rows.Close()
			return value, err
		}
		line.Required, err = m.ParseDecimal(required)
		if err != nil {
			rows.Close()
			return value, err
		}
		line.Reserved, err = m.ParseDecimal(reserved)
		if err != nil {
			rows.Close()
			return value, err
		}
		line.Consumed, err = m.ParseDecimal(consumed)
		if err != nil {
			rows.Close()
			return value, err
		}
		remaining, subtractErr := line.Required.Subtract(line.Consumed)
		if subtractErr != nil {
			rows.Close()
			return value, subtractErr
		}
		line.Shortage, err = remaining.Subtract(line.Reserved)
		if err != nil {
			rows.Close()
			return value, err
		}
		if !line.Shortage.IsZero() {
			ready = false
		}
		if jobStatus == "completed" {
			line.Shortage = m.Decimal{}
		}
		value.Lines = append(value.Lines, line)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return value, err
	}
	value.Ready = ready && len(value.Lines) > 0 && jobStatus == "queued"
	return value, nil
}
func (d *Database) GetPlan(ctx context.Context, actor, id string) (m.Plan, error) {
	tx, err := d.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return m.Plan{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "staff"); err != nil {
		return m.Plan{}, err
	}
	value, err := loadPlan(ctx, tx, id)
	if err != nil {
		return value, err
	}
	return value, tx.Commit(ctx)
}
func (d *Database) ApplyPlan(ctx context.Context, actor, id string, in m.PlanInput, reserveOnly bool) (m.Plan, error) {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return m.Plan{}, err
	}
	defer tx.Rollback(ctx)
	if err = inventoryActor(ctx, tx, actor, "worker"); err != nil {
		return m.Plan{}, err
	}
	var requestStatus string
	// Lock hierarchy: request, job, then all material rows sorted by UUID.
	err = tx.QueryRow(ctx, `SELECT r.status FROM job j JOIN job_request r ON r.id=j.request_id WHERE j.id=$1 FOR UPDATE OF r`, id).Scan(&requestStatus)
	if err != nil {
		return m.Plan{}, inventoryError(err)
	}
	var version int64
	var jobStatus string
	if err = tx.QueryRow(ctx, `SELECT material_plan_version,status FROM job WHERE id=$1 FOR UPDATE`, id).Scan(&version, &jobStatus); err != nil {
		return m.Plan{}, inventoryError(err)
	}
	payload, err := json.Marshal(struct {
		ReserveOnly bool        `json:"reserve_only"`
		Input       m.PlanInput `json:"input"`
	}{reserveOnly, in})
	if err != nil {
		return m.Plan{}, err
	}
	var same bool
	var stored []byte
	err = tx.QueryRow(ctx, `SELECT actor_id=$3::uuid AND payload=$4::jsonb,result FROM job_material_command WHERE job_id=$1 AND idempotency_key=$2`, id, in.IdempotencyKey, actor, payload).Scan(&same, &stored)
	if err == nil {
		if !same {
			return m.Plan{}, m.ErrConflict
		}
		var v m.Plan
		if err = json.Unmarshal(stored, &v); err != nil {
			return v, err
		}
		return v, tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return m.Plan{}, err
	}
	if version != in.ExpectedVersion || jobStatus != "queued" || requestStatus != "ongoing" {
		return m.Plan{}, m.ErrConflict
	}
	old, err := loadPlan(ctx, tx, id)
	if err != nil {
		return old, err
	}
	prior := map[string]m.PlanLine{}
	targets := map[string]m.Decimal{}
	for _, line := range old.Lines {
		prior[line.MaterialID] = line
		targets[line.MaterialID] = m.Decimal{}
	}
	if reserveOnly {
		for _, line := range old.Lines {
			targets[line.MaterialID] = line.Required
		}
	} else {
		for _, line := range in.Lines {
			if line.Quantity == nil {
				return m.Plan{}, req.ErrInvalidInput
			}
			targets[line.MaterialID] = *line.Quantity
		}
	}
	keys := make([]string, 0, len(targets))
	for key := range targets {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	materials := map[string]m.Material{}
	if len(keys) > 0 {
		rows, err := tx.Query(ctx, `SELECT `+materialFields+` FROM material m JOIN unit u ON u.id=m.unit_id WHERE m.id=ANY($1::uuid[]) ORDER BY m.id FOR UPDATE OF m`, keys)
		if err != nil {
			return m.Plan{}, err
		}
		for rows.Next() {
			v, err := materialRow(rows)
			if err != nil {
				rows.Close()
				return m.Plan{}, err
			}
			materials[v.ID] = v
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return m.Plan{}, err
		}
		if len(materials) != len(keys) {
			return m.Plan{}, req.ErrNotFound
		}
	}
	for _, key := range keys {
		material := materials[key]
		previous := prior[key]
		target := targets[key]
		if !material.Active {
			if _, err := previous.Required.Subtract(target); err != nil {
				return m.Plan{}, m.ErrConflict
			}
			material.Available = m.Decimal{}
		}
		reserved, _, err := m.Allocate(target, previous.Reserved, material.Available)
		if err != nil {
			return m.Plan{}, err
		}
		// Release before reducing the plan; expand the plan before adding reservations.
		_, reducing := target.Subtract(previous.Reserved)
		if reducing != nil {
			if _, err = tx.Exec(ctx, `UPDATE material_reservation SET qty_reserved=$3::numeric WHERE job_id=$1 AND material_id=$2`, id, key, reserved.String()); err != nil {
				return m.Plan{}, inventoryError(err)
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO job_material_requirement(job_id,material_id,required_qty) VALUES($1,$2,$3::numeric) ON CONFLICT(job_id,material_id) DO UPDATE SET required_qty=EXCLUDED.required_qty`, id, key, target.String())
		if err != nil {
			return m.Plan{}, inventoryError(err)
		}
		if reducing == nil {
			_, err = tx.Exec(ctx, `INSERT INTO material_reservation(job_id,material_id,qty_reserved) VALUES($1,$2,$3::numeric) ON CONFLICT(job_id,material_id) DO UPDATE SET qty_reserved=EXCLUDED.qty_reserved`, id, key, reserved.String())
			if err != nil {
				return m.Plan{}, inventoryError(err)
			}
		}
	}
	if _, err = tx.Exec(ctx, `UPDATE job SET material_plan_version=material_plan_version+1 WHERE id=$1`, id); err != nil {
		return m.Plan{}, err
	}
	value, err := loadPlan(ctx, tx, id)
	if err != nil {
		return value, err
	}
	result, err := json.Marshal(value)
	if err != nil {
		return value, err
	}
	_, err = tx.Exec(ctx, `INSERT INTO job_material_command(id,job_id,actor_id,idempotency_key,expected_version,version,payload,result) VALUES($1,$2,$3,$4,$5,$6,$7::jsonb,$8::jsonb)`, uuid.NewString(), id, actor, in.IdempotencyKey, version, version+1, payload, result)
	if err != nil {
		return value, inventoryError(err)
	}
	return value, tx.Commit(ctx)
}
