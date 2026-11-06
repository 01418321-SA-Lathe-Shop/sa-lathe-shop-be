-- ER alignment, phase 2: store the derived values the ER diagram has as columns. Each is maintained by the
-- database itself and may only ever hold the value computed from the source tables, so it cannot drift.
-- Source tables stay authoritative; history guards still block every other edit.

-- INVOICE.status ---------------------------------------------------------------------------------
ALTER TABLE invoice ADD COLUMN status text NOT NULL DEFAULT 'unpaid' CHECK (status IN ('unpaid','partial','paid'));
CREATE FUNCTION invoice_paid_status(inv uuid) RETURNS text LANGUAGE sql STABLE AS $$
 SELECT CASE WHEN p.paid >= i.amount THEN 'paid' WHEN p.paid > 0 THEN 'partial' ELSE 'unpaid' END
 FROM invoice i, LATERAL (SELECT COALESCE(sum(a.amount),0) AS paid FROM payment_allocation a WHERE a.invoice_id=i.id) p WHERE i.id=inv
$$;
ALTER TABLE invoice DISABLE TRIGGER invoice_immutable;
UPDATE invoice SET status=invoice_paid_status(id);
ALTER TABLE invoice ENABLE TRIGGER invoice_immutable;
DROP TRIGGER invoice_immutable ON invoice;
CREATE FUNCTION invoice_update_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' OR (to_jsonb(NEW)-'status') IS DISTINCT FROM (to_jsonb(OLD)-'status') OR NEW.status IS DISTINCT FROM invoice_paid_status(NEW.id) THEN
  RAISE EXCEPTION 'Procurement history is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER invoice_immutable BEFORE DELETE OR UPDATE ON invoice FOR EACH ROW EXECUTE FUNCTION invoice_update_guard();
CREATE FUNCTION sync_invoice_status() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN UPDATE invoice SET status=invoice_paid_status(NEW.invoice_id) WHERE id=NEW.invoice_id; RETURN NULL; END $$;
CREATE TRIGGER payment_allocation_sync_invoice AFTER INSERT ON payment_allocation FOR EACH ROW EXECUTE FUNCTION sync_invoice_status();

-- MATERIAL_ORDER_ITEM received / damaged quantities, status and dates --------------------------------
ALTER TABLE material_order_item
 ADD COLUMN qty_received numeric NOT NULL DEFAULT 0,
 ADD COLUMN qty_damaged numeric NOT NULL DEFAULT 0,
 ADD COLUMN status text NOT NULL DEFAULT 'ordered' CHECK (status IN ('ordered','preparing','in_transit','received','damaged','cancelled')),
 ADD COLUMN expected_at timestamptz,
 ADD COLUMN received_at timestamptz,
 ADD COLUMN created_at timestamptz NOT NULL DEFAULT now(),
 ADD COLUMN edited_at timestamptz;
-- qty_received counts everything that arrived (good + damaged); status is received once good quantity covers the
-- order, damaged once the ordered quantity has arrived but part of it was damaged. preparing and in_transit have no source.
CREATE FUNCTION order_item_state(o uuid, m uuid, OUT qty_received numeric, OUT qty_damaged numeric, OUT status text, OUT received_at timestamptz)
LANGUAGE plpgsql STABLE AS $$
DECLARE good numeric; ordered numeric; cancelled boolean;
BEGIN
 SELECT COALESCE(sum(l.good),0), COALESCE(sum(l.damaged),0), max(r.created_at) INTO good, qty_damaged, received_at
 FROM material_receipt_line l JOIN material_receipt r ON r.id=l.receipt_id WHERE l.order_id=o AND l.material_id=m;
 qty_received := good + qty_damaged;
 SELECT i.qty_ordered, od.status='cancelled' INTO ordered, cancelled FROM material_order_item i JOIN material_order od ON od.id=i.order_id WHERE i.order_id=o AND i.material_id=m;
 status := CASE WHEN cancelled THEN 'cancelled' WHEN good >= ordered THEN 'received' WHEN qty_received >= ordered AND qty_damaged > 0 THEN 'damaged' ELSE 'ordered' END;
END $$;
ALTER TABLE material_order_item DISABLE TRIGGER material_order_item_immutable;
UPDATE material_order_item i SET
 expected_at=(SELECT o.expected_at FROM material_order o WHERE o.id=i.order_id),
 created_at=(SELECT o.created_at FROM material_order o WHERE o.id=i.order_id),
 (qty_received,qty_damaged,status,received_at)=(SELECT s.qty_received,s.qty_damaged,s.status,s.received_at FROM order_item_state(i.order_id,i.material_id) s);
ALTER TABLE material_order_item ENABLE TRIGGER material_order_item_immutable;
DROP TRIGGER material_order_item_immutable ON material_order_item;
CREATE FUNCTION order_item_update_guard() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE s record;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Procurement history is immutable' USING ERRCODE='23514'; END IF;
 SELECT * INTO s FROM order_item_state(NEW.order_id,NEW.material_id);
 -- total_amount is a generated column and is still NULL in NEW inside a BEFORE trigger.
 IF (to_jsonb(NEW)-'qty_received'-'qty_damaged'-'status'-'received_at'-'edited_at'-'total_amount') IS DISTINCT FROM (to_jsonb(OLD)-'qty_received'-'qty_damaged'-'status'-'received_at'-'edited_at'-'total_amount')
 OR (NEW.qty_received,NEW.qty_damaged,NEW.status,NEW.received_at) IS DISTINCT FROM (s.qty_received,s.qty_damaged,s.status,s.received_at) THEN
  RAISE EXCEPTION 'Procurement history is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER material_order_item_immutable BEFORE DELETE OR UPDATE ON material_order_item FOR EACH ROW EXECUTE FUNCTION order_item_update_guard();
CREATE FUNCTION sync_order_item() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_TABLE_NAME='material_receipt_line' THEN
  UPDATE material_order_item i SET (qty_received,qty_damaged,status,received_at,edited_at)=(SELECT s.qty_received,s.qty_damaged,s.status,s.received_at,now() FROM order_item_state(i.order_id,i.material_id) s)
  WHERE i.order_id=NEW.order_id AND i.material_id=NEW.material_id;
 ELSE
  UPDATE material_order_item i SET (qty_received,qty_damaged,status,received_at,edited_at)=(SELECT s.qty_received,s.qty_damaged,s.status,s.received_at,now() FROM order_item_state(i.order_id,i.material_id) s)
  WHERE i.order_id=NEW.id;
 END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER material_receipt_line_sync_item AFTER INSERT ON material_receipt_line FOR EACH ROW EXECUTE FUNCTION sync_order_item();
CREATE TRIGGER material_order_sync_items AFTER UPDATE OF status ON material_order FOR EACH ROW WHEN (NEW.status='cancelled' AND OLD.status IS DISTINCT FROM NEW.status) EXECUTE FUNCTION sync_order_item();

-- MATERIAL_ORDER supplier payment details (copied from the settlement when it is recorded) ---------------
ALTER TABLE material_order ADD COLUMN payment_method text, ADD COLUMN paid_at timestamptz, ADD COLUMN payment_proof_url text;
ALTER TABLE material_order DISABLE TRIGGER material_order_guard;
UPDATE material_order o SET payment_method=s.method, paid_at=s.paid_at, payment_proof_url=e.object_key
FROM supplier_settlement s LEFT JOIN order_evidence e ON e.id=s.evidence_id WHERE s.order_id=o.id;
-- The UPDATE above queues the deferred material_order_consistent trigger; run it now or ALTER TABLE
-- fails with "pending trigger events" on any database that has settled supplier orders.
SET CONSTRAINTS ALL IMMEDIATE;
ALTER TABLE material_order ENABLE TRIGGER material_order_guard;

-- JOB estimated / actual cost and last edit ------------------------------------------------------------
-- estimated_cost: quotation material + labor cost. actual_cost: actual labor + issued material, set when the job completes
-- (cleared if a customer return reopens it). estimated_time / actual_time are not added: no stored source.
ALTER TABLE job ADD COLUMN estimated_cost numeric(18,2), ADD COLUMN actual_cost numeric(18,2), ADD COLUMN edited_at timestamptz;
UPDATE job j SET estimated_cost=qc.material_cost+qc.labor_cost FROM quotation_cost qc WHERE qc.quotation_id=j.quotation_id;
UPDATE job j SET actual_cost=COALESCE(j.actual_labor_cost,0)+COALESCE((SELECT sum(u.total_amount) FROM job_material_usage u WHERE u.job_id=j.id),0) WHERE j.status='completed';
CREATE FUNCTION job_er_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN SELECT material_cost+labor_cost INTO NEW.estimated_cost FROM quotation_cost WHERE quotation_id=NEW.quotation_id; RETURN NEW; END $$;
CREATE TRIGGER job_er_insert BEFORE INSERT ON job FOR EACH ROW EXECUTE FUNCTION job_er_insert();
CREATE FUNCTION job_er_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.status='completed' THEN
  NEW.actual_cost := COALESCE(NEW.actual_labor_cost,0)+COALESCE((SELECT sum(u.total_amount) FROM job_material_usage u WHERE u.job_id=NEW.id),0);
 ELSE NEW.actual_cost := NULL; END IF;
 NEW.edited_at := now();
 RETURN NEW;
END $$;
CREATE TRIGGER job_er_update BEFORE UPDATE ON job FOR EACH ROW EXECUTE FUNCTION job_er_update();

-- The order guard must let the new payment columns change together with payment_status.
CREATE OR REPLACE FUNCTION public.procurement_order_guard()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Orders cannot be deleted' USING ERRCODE='23514'; END IF;
 IF (to_jsonb(NEW)-'status'-'supplier_response'-'version'-'credit_days'-'full_received_at'-'payment_status'-'payment_method'-'paid_at'-'payment_proof_url') IS DISTINCT FROM (to_jsonb(OLD)-'status'-'supplier_response'-'version'-'credit_days'-'full_received_at'-'payment_status'-'payment_method'-'paid_at'-'payment_proof_url')
 OR NEW.version<>OLD.version+1 OR OLD.status='cancelled' OR (OLD.credit_days IS NOT NULL AND NEW.credit_days IS DISTINCT FROM OLD.credit_days)
 OR (OLD.full_received_at IS NOT NULL AND NEW.full_received_at IS DISTINCT FROM OLD.full_received_at) THEN RAISE EXCEPTION 'Invalid order edit' USING ERRCODE='23514'; END IF;
 IF NEW.supplier_response IS DISTINCT FROM OLD.supplier_response AND NOT (OLD.status='ordered' AND OLD.supplier_response='pending' AND ((NEW.supplier_response='accepted' AND NEW.status='ordered') OR (NEW.supplier_response='rejected' AND NEW.status='cancelled'))) THEN RAISE EXCEPTION 'Invalid supplier response' USING ERRCODE='23514'; END IF;
 IF NEW.status='cancelled' AND EXISTS(SELECT 1 FROM material_receipt WHERE order_id=OLD.id) THEN RAISE EXCEPTION 'Cannot cancel after any receipt' USING ERRCODE='23514'; END IF;
 IF (NEW.status IN ('partially_received','received','damaged') AND NEW.supplier_response<>'accepted')
  OR (OLD.status='received' AND NEW.status<>'received')
  OR (OLD.status='damaged' AND NEW.status NOT IN ('damaged','received'))
  OR (OLD.status='partially_received' AND NEW.status NOT IN ('partially_received','received','damaged')) THEN RAISE EXCEPTION 'Invalid receipt transition' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $function$;

