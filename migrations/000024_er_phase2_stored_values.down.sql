DROP TRIGGER job_er_update ON job;
DROP TRIGGER job_er_insert ON job;
DROP FUNCTION job_er_update();
DROP FUNCTION job_er_insert();
ALTER TABLE job DROP COLUMN estimated_cost, DROP COLUMN actual_cost, DROP COLUMN edited_at;
CREATE OR REPLACE FUNCTION public.procurement_order_guard()
 RETURNS trigger
 LANGUAGE plpgsql
AS $function$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Orders cannot be deleted' USING ERRCODE='23514'; END IF;
 IF (to_jsonb(NEW)-'status'-'supplier_response'-'version'-'credit_days'-'full_received_at'-'payment_status') IS DISTINCT FROM (to_jsonb(OLD)-'status'-'supplier_response'-'version'-'credit_days'-'full_received_at'-'payment_status')
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

ALTER TABLE material_order DROP COLUMN payment_method, DROP COLUMN paid_at, DROP COLUMN payment_proof_url;
DROP TRIGGER material_order_sync_items ON material_order;
DROP TRIGGER material_receipt_line_sync_item ON material_receipt_line;
DROP FUNCTION sync_order_item();
DROP TRIGGER material_order_item_immutable ON material_order_item;
CREATE TRIGGER material_order_item_immutable BEFORE DELETE OR UPDATE ON material_order_item FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
DROP FUNCTION order_item_update_guard();
ALTER TABLE material_order_item DROP COLUMN qty_received, DROP COLUMN qty_damaged, DROP COLUMN status, DROP COLUMN expected_at, DROP COLUMN received_at, DROP COLUMN created_at, DROP COLUMN edited_at;
DROP FUNCTION order_item_state(uuid, uuid);
DROP TRIGGER payment_allocation_sync_invoice ON payment_allocation;
DROP FUNCTION sync_invoice_status();
DROP TRIGGER invoice_immutable ON invoice;
CREATE TRIGGER invoice_immutable BEFORE DELETE OR UPDATE ON invoice FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
DROP FUNCTION invoice_update_guard();
ALTER TABLE invoice DROP COLUMN status;
DROP FUNCTION invoice_paid_status(uuid);
