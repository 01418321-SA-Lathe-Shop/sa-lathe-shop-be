DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM production_event) OR EXISTS(SELECT 1 FROM job_material_usage) OR EXISTS(SELECT 1 FROM job_time_entry) OR EXISTS(SELECT 1 FROM job_photo) OR EXISTS(SELECT 1 FROM job WHERE production_version>0 OR started_at IS NOT NULL OR completed_at IS NOT NULL OR actual_labor_cost IS NOT NULL) THEN
  RAISE EXCEPTION 'Production data exists: use a planned recovery migration, never discard usage/history';
 END IF;
END $$;
DROP TRIGGER production_job_guard ON job;
DROP TRIGGER production_job_consistent ON job;
DROP TRIGGER consumed_reservation_consistent ON material_reservation;
CREATE OR REPLACE FUNCTION apply_material_reservation() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE needed numeric; previous numeric:=0; m material; j job;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Release reservations explicitly' USING ERRCODE='23514';END IF;
 IF TG_OP='UPDATE' THEN
  IF (OLD.job_id,OLD.material_id) IS DISTINCT FROM (NEW.job_id,NEW.material_id) THEN RAISE EXCEPTION 'Reservation identity is immutable' USING ERRCODE='23514';END IF;
  previous:=OLD.qty_reserved;
 END IF;
 SELECT * INTO j FROM job WHERE id=NEW.job_id FOR UPDATE;
 SELECT * INTO m FROM material WHERE id=NEW.material_id FOR UPDATE;
 SELECT required_qty INTO needed FROM job_material_requirement WHERE job_id=NEW.job_id AND material_id=NEW.material_id;
 IF j.status<>'queued' OR needed IS NULL OR NEW.qty_reserved>needed OR (NOT m.is_active AND NEW.qty_reserved>previous) THEN
  RAISE EXCEPTION 'Invalid reservation/queued job' USING ERRCODE='23514';
 END IF;
 UPDATE material SET qty_reserved=qty_reserved+NEW.qty_reserved-previous,
 version=version+CASE WHEN NEW.qty_reserved<>previous THEN 1 ELSE 0 END,edited_at=clock_timestamp() WHERE id=NEW.material_id;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION apply_stock_movement() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE m material; next_qty numeric; next_cost numeric;
BEGIN
 SELECT * INTO m FROM material WHERE id=NEW.material_id FOR UPDATE;
 IF m.id IS NULL OR m.version<>NEW.expected_version OR NEW.before_quantity<>m.qty_on_hand OR NEW.before_cost<>m.avg_cost
 OR NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.actor_id AND is_active AND (role='admin' OR (NEW.mode='purchase_receive' AND role='worker'))) THEN
  RAISE EXCEPTION 'Stock snapshot/permission conflict' USING ERRCODE='23514'; END IF;
 IF NEW.mode IN ('addition','purchase_receive') THEN
  IF NOT m.is_active THEN RAISE EXCEPTION 'Material is inactive' USING ERRCODE='23514'; END IF;
  IF NEW.mode='purchase_receive' AND NOT EXISTS(SELECT 1 FROM material_receipt_line l JOIN material_receipt r ON r.id=l.receipt_id JOIN material_order_item i ON (i.order_id,i.material_id)=(l.order_id,l.material_id)
   WHERE l.id=NEW.receipt_line_id AND l.material_id=m.id AND l.good=NEW.quantity AND i.unit_cost=NEW.unit_cost AND r.actor_id=NEW.actor_id) THEN
   RAISE EXCEPTION 'Receipt ledger mismatch' USING ERRCODE='23514'; END IF;
  next_qty:=m.qty_on_hand+NEW.quantity;next_cost:=round((m.qty_on_hand*m.avg_cost+NEW.quantity*NEW.unit_cost)/next_qty,6);
 ELSE next_qty:=NEW.quantity;next_cost:=m.avg_cost;END IF;
 IF NEW.after_quantity<>next_qty OR NEW.after_cost<>next_cost OR next_qty<m.qty_reserved THEN RAISE EXCEPTION 'Invalid stock effect' USING ERRCODE='23514'; END IF;
 UPDATE material SET qty_on_hand=next_qty,avg_cost=next_cost,version=NEW.version,edited_at=clock_timestamp() WHERE id=m.id;RETURN NEW;
END $$;
ALTER TABLE stock_movement DROP CONSTRAINT stock_movement_usage_check,DROP CONSTRAINT stock_movement_effect_check,DROP CONSTRAINT stock_movement_mode_check,DROP COLUMN job_usage_id;
ALTER TABLE stock_movement ADD CONSTRAINT stock_movement_mode_check CHECK(mode IN ('addition','correction','purchase_receive'));
ALTER TABLE stock_movement ADD CONSTRAINT stock_movement_effect_check CHECK((mode IN ('addition','purchase_receive') AND quantity>0 AND unit_cost IS NOT NULL) OR (mode='correction' AND unit_cost IS NULL));
DROP TABLE job_photo,job_time_entry,job_material_usage,production_event;
ALTER TABLE material_reservation DROP COLUMN status,DROP COLUMN qty_consumed;
ALTER TABLE material_reservation ADD COLUMN status text GENERATED ALWAYS AS (CASE WHEN qty_reserved>0 THEN 'reserved' ELSE 'released' END) STORED;
ALTER TABLE job DROP COLUMN actual_labor_cost,DROP COLUMN completed_at,DROP COLUMN started_at,DROP COLUMN production_version;
DROP FUNCTION guard_production_usage(),check_production_usage(),check_consumed_reservation(),guard_production_job(),check_production_job();
