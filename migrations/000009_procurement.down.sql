-- Never silently discard purchasing commitments or supplier audit history.
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM supplier) OR EXISTS(SELECT 1 FROM material_order) THEN
  RAISE EXCEPTION 'Procurement data exists: export and plan an explicit rollback first';
 END IF;
END $$;
DROP TABLE material_order_event,material_order_item,material_order_job,material_order,supplier_audit,supplier;
DROP FUNCTION procurement_order_check(),procurement_order_guard(),procurement_child_insert(),procurement_immutable();
