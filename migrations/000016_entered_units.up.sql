-- What a person typed, kept beside the base-unit quantity that drives stock. NULL means base unit.
ALTER TABLE stock_movement
 ADD COLUMN entered_qty numeric(18,6),ADD COLUMN entered_unit_name text,ADD COLUMN factor_used numeric(18,6),
 ADD CONSTRAINT stock_movement_entered_check CHECK((entered_qty IS NULL AND entered_unit_name IS NULL AND factor_used IS NULL) OR (entered_unit_name IS NOT NULL AND entered_qty*factor_used=quantity));
ALTER TABLE job_material_requirement
 ADD COLUMN entered_qty numeric(18,6),ADD COLUMN entered_unit_name text,ADD COLUMN factor_used numeric(18,6),
 ADD CONSTRAINT job_material_requirement_entered_check CHECK((entered_qty IS NULL AND entered_unit_name IS NULL AND factor_used IS NULL) OR (entered_unit_name IS NOT NULL AND entered_qty*factor_used=required_qty));
ALTER TABLE job_material_usage
 ADD COLUMN entered_qty numeric(18,6),ADD COLUMN entered_unit_name text,ADD COLUMN factor_used numeric(18,6),
 ADD CONSTRAINT job_material_usage_entered_check CHECK((entered_qty IS NULL AND entered_unit_name IS NULL AND factor_used IS NULL) OR (entered_unit_name IS NOT NULL AND entered_qty*factor_used=quantity));
