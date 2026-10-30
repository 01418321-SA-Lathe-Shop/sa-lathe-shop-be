ALTER TABLE job_material_usage DROP COLUMN entered_qty,DROP COLUMN entered_unit_name,DROP COLUMN factor_used;
ALTER TABLE job_material_requirement DROP COLUMN entered_qty,DROP COLUMN entered_unit_name,DROP COLUMN factor_used;
ALTER TABLE stock_movement DROP COLUMN entered_qty,DROP COLUMN entered_unit_name,DROP COLUMN factor_used;
