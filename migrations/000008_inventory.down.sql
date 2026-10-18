DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM material) OR EXISTS(SELECT 1 FROM unit) OR EXISTS(SELECT 1 FROM job_material_command) THEN
  RAISE EXCEPTION 'Inventory data exists; use a backed-up forward migration';
 END IF;
END $$;
DROP TRIGGER material_reservation_apply ON material_reservation;
DROP FUNCTION apply_material_reservation();
DROP TRIGGER material_requirement_guard ON job_material_requirement;
DROP FUNCTION guard_material_requirement();
DROP TRIGGER stock_movement_apply ON stock_movement;
DROP FUNCTION apply_stock_movement();
DROP TRIGGER material_balance_guard ON material;
DROP FUNCTION protect_material_balance();
DROP TABLE job_material_command;
DROP TABLE material_reservation;
DROP TABLE job_material_requirement;
DROP TABLE stock_movement;
DROP TABLE material_catalog_audit;
DROP FUNCTION inventory_audit_immutable();
DROP TABLE material;
DROP TABLE unit;
ALTER TABLE job DROP COLUMN material_plan_version;
