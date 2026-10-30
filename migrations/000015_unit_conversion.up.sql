-- Per-material entry units. Stock stays in the material's base unit: 1 unit = factor base units.
CREATE TABLE unit_conversion (
 id uuid PRIMARY KEY,
 material_id uuid NOT NULL REFERENCES material(id) ON DELETE RESTRICT,
 unit_id uuid NOT NULL REFERENCES unit(id) ON DELETE RESTRICT,
 factor numeric(18,6) NOT NULL CHECK(factor>0 AND factor<>'NaN'::numeric),
 is_active boolean NOT NULL DEFAULT true,
 created_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 deactivated_by uuid REFERENCES account(id) ON DELETE RESTRICT,
 deactivated_at timestamptz,
 CHECK(is_active=(deactivated_at IS NULL) AND is_active=(deactivated_by IS NULL))
);
CREATE UNIQUE INDEX unit_conversion_one_active ON unit_conversion(material_id,unit_id) WHERE is_active;
CREATE INDEX unit_conversion_material ON unit_conversion(material_id,created_at,id);

CREATE FUNCTION protect_unit_conversion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Unit conversions are retained' USING ERRCODE='23514'; END IF;
 IF TG_OP='INSERT' THEN
  IF EXISTS(SELECT 1 FROM material WHERE id=NEW.material_id AND unit_id=NEW.unit_id) THEN
   RAISE EXCEPTION 'Base unit needs no conversion' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
 END IF;
 -- Past records rely on a used factor, so the only permitted change is deactivation.
 IF NOT OLD.is_active OR NEW.is_active OR (NEW.id,NEW.material_id,NEW.unit_id,NEW.factor,NEW.created_by,NEW.created_at) IS DISTINCT FROM (OLD.id,OLD.material_id,OLD.unit_id,OLD.factor,OLD.created_by,OLD.created_at) THEN
  RAISE EXCEPTION 'Unit conversions can only be deactivated' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER unit_conversion_protected BEFORE INSERT OR UPDATE OR DELETE ON unit_conversion FOR EACH ROW EXECUTE FUNCTION protect_unit_conversion();
