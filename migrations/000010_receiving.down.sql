DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM material_receipt) OR EXISTS(SELECT 1 FROM order_evidence) OR EXISTS(SELECT 1 FROM supplier_settlement) OR EXISTS(SELECT 1 FROM material_order WHERE credit_days IS NOT NULL) OR EXISTS(SELECT 1 FROM material_order_event WHERE action IN ('receive','claim','credit_terms','settle')) THEN
  RAISE EXCEPTION 'UC7 data exists: plan an explicit recovery migration; no destructive rollback';
 END IF;
END $$;
CREATE OR REPLACE FUNCTION apply_stock_movement() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE m material; next_qty numeric; next_cost numeric;
BEGIN
 SELECT * INTO m FROM material WHERE id=NEW.material_id FOR UPDATE;
 IF m.id IS NULL OR NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.actor_id AND role='admin' AND is_active)
 OR m.version<>NEW.expected_version OR NEW.before_quantity<>m.qty_on_hand OR NEW.before_cost<>m.avg_cost THEN
  RAISE EXCEPTION 'Stock snapshot/permission conflict' USING ERRCODE='23514';
 END IF;
 IF NEW.mode='addition' THEN
  IF NOT m.is_active THEN RAISE EXCEPTION 'Material is inactive' USING ERRCODE='23514'; END IF;
  next_qty:=m.qty_on_hand+NEW.quantity;
  next_cost:=round((m.qty_on_hand*m.avg_cost+NEW.quantity*NEW.unit_cost)/next_qty,6);
 ELSE next_qty:=NEW.quantity;next_cost:=m.avg_cost;
 END IF;
 IF NEW.after_quantity<>next_qty OR NEW.after_cost<>next_cost OR next_qty<m.qty_reserved THEN
  RAISE EXCEPTION 'Invalid stock effect' USING ERRCODE='23514';
 END IF;
 UPDATE material SET qty_on_hand=next_qty,avg_cost=next_cost,version=NEW.version,edited_at=clock_timestamp() WHERE id=m.id;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION procurement_order_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Orders cannot be deleted' USING ERRCODE='23514'; END IF;
 IF (to_jsonb(NEW)-'status'-'supplier_response'-'version') IS DISTINCT FROM (to_jsonb(OLD)-'status'-'supplier_response'-'version')
  OR NEW.version<>OLD.version+1 OR OLD.status<>'ordered' THEN
  RAISE EXCEPTION 'Invalid or non-versioned order edit' USING ERRCODE='23514';
 END IF;
 IF NOT ((OLD.supplier_response='pending' AND NEW.supplier_response='accepted' AND NEW.status='ordered')
  OR (OLD.supplier_response='pending' AND NEW.supplier_response='rejected' AND NEW.status='cancelled')
  OR (NEW.supplier_response=OLD.supplier_response AND NEW.status='cancelled')) THEN
  RAISE EXCEPTION 'Invalid supplier order transition' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION procurement_order_check() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE v material_order; line_count int; line_sum numeric; event_action text;
BEGIN
 SELECT * INTO v FROM material_order WHERE id=NEW.id;
 SELECT count(*),sum(total_amount) INTO line_count,line_sum FROM material_order_item WHERE order_id=v.id;
 IF line_count NOT BETWEEN 1 AND 100 OR line_sum IS DISTINCT FROM v.total_amount THEN
  RAISE EXCEPTION 'Order total must equal rounded item totals' USING ERRCODE='23514';
 END IF;
 SELECT action INTO event_action FROM material_order_event WHERE order_id=v.id AND version=v.version;
 IF event_action IS NULL OR (v.version=1 AND event_action<>'place')
  OR (v.version>1 AND ((v.status='ordered' AND event_action<>'accept')
   OR (v.status='cancelled' AND v.supplier_response='rejected' AND event_action<>'reject')
   OR (v.status='cancelled' AND v.supplier_response<>'rejected' AND event_action<>'cancel'))) THEN
  RAISE EXCEPTION 'Order transition requires matching audit event' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
ALTER TABLE stock_movement DROP CONSTRAINT stock_movement_mode_check,DROP CONSTRAINT stock_movement_effect_check,DROP CONSTRAINT stock_movement_receipt_check,DROP COLUMN receipt_line_id;
ALTER TABLE stock_movement ADD CONSTRAINT stock_movement_mode_check CHECK(mode IN ('addition','correction'));
ALTER TABLE stock_movement ADD CONSTRAINT stock_movement_effect_check CHECK((mode='addition' AND quantity>0 AND unit_cost IS NOT NULL) OR (mode='correction' AND unit_cost IS NULL));
ALTER TABLE material_order DROP CONSTRAINT material_order_full_receipt_check,DROP COLUMN full_received_at,DROP COLUMN credit_days,DROP CONSTRAINT material_order_status_check;
ALTER TABLE material_order ADD CONSTRAINT material_order_status_check CHECK(status IN ('ordered','cancelled'));
ALTER TABLE material_order_event DROP CONSTRAINT material_order_event_action_check;
ALTER TABLE material_order_event ADD CONSTRAINT material_order_event_action_check CHECK(action IN ('place','accept','reject','cancel'));
DROP TABLE supplier_settlement,order_evidence,material_claim,material_receipt_line,material_receipt;
DROP FUNCTION guard_material_receipt(),guard_receipt_line(),guard_material_claim(),guard_order_evidence(),guard_supplier_settlement(),check_material_receipt();
