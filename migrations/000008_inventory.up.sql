ALTER TABLE job ADD COLUMN material_plan_version bigint NOT NULL DEFAULT 0 CHECK(material_plan_version>=0);
CREATE TABLE unit (
 id uuid PRIMARY KEY,
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 1 AND 50),
 created_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE UNIQUE INDEX unit_name_unique ON unit(lower(name));
CREATE TABLE material (
 id uuid PRIMARY KEY,
 name text NOT NULL CHECK(length(btrim(name)) BETWEEN 1 AND 200),
 unit_id uuid NOT NULL REFERENCES unit(id) ON DELETE RESTRICT,
 is_active boolean NOT NULL DEFAULT true,
 qty_on_hand numeric(18,6) NOT NULL DEFAULT 0 CHECK(qty_on_hand>=0 AND qty_on_hand<>'NaN'::numeric),
 qty_reserved numeric(18,6) NOT NULL DEFAULT 0 CHECK(qty_reserved>=0 AND qty_reserved<>'NaN'::numeric AND qty_reserved<=qty_on_hand),
 avg_cost numeric(18,6) NOT NULL DEFAULT 0 CHECK(avg_cost>=0 AND avg_cost<>'NaN'::numeric),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 edited_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE UNIQUE INDEX material_name_unique ON material(lower(name));
CREATE TABLE material_catalog_audit (
 id uuid PRIMARY KEY, material_id uuid NOT NULL REFERENCES material(id) ON DELETE RESTRICT,
 actor_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 before_name text NOT NULL,after_name text NOT NULL,before_active boolean NOT NULL,after_active boolean NOT NULL,
 expected_version bigint NOT NULL,version bigint NOT NULL CHECK(version=expected_version+1),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE stock_movement (
 id uuid PRIMARY KEY,material_id uuid NOT NULL REFERENCES material(id) ON DELETE RESTRICT,
 actor_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,idempotency_key uuid NOT NULL,
 mode text NOT NULL CHECK(mode IN ('addition','correction')),
 quantity numeric(18,6) NOT NULL CHECK(quantity>=0 AND quantity<>'NaN'::numeric),
 unit_cost numeric(18,6) CHECK(unit_cost>=0 AND unit_cost<>'NaN'::numeric),
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 2000),
 expected_version bigint NOT NULL CHECK(expected_version>0),version bigint NOT NULL CHECK(version=expected_version+1),
 before_quantity numeric(18,6) NOT NULL,after_quantity numeric(18,6) NOT NULL,
 before_cost numeric(18,6) NOT NULL,after_cost numeric(18,6) NOT NULL,
 payload jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 UNIQUE(material_id,idempotency_key),
 CHECK((mode='addition' AND quantity>0 AND unit_cost IS NOT NULL) OR (mode='correction' AND unit_cost IS NULL))
);
CREATE INDEX stock_movement_history ON stock_movement(material_id,created_at DESC,id);
CREATE TABLE job_material_requirement (
 job_id uuid NOT NULL REFERENCES job(id) ON DELETE RESTRICT,
 material_id uuid NOT NULL REFERENCES material(id) ON DELETE RESTRICT,
 required_qty numeric(18,6) NOT NULL CHECK(required_qty>=0 AND required_qty<>'NaN'::numeric),
 PRIMARY KEY(job_id,material_id)
);
CREATE TABLE material_reservation (
 job_id uuid NOT NULL, material_id uuid NOT NULL,
 qty_reserved numeric(18,6) NOT NULL CHECK(qty_reserved>=0 AND qty_reserved<>'NaN'::numeric),
 status text GENERATED ALWAYS AS (CASE WHEN qty_reserved>0 THEN 'reserved' ELSE 'released' END) STORED,
 PRIMARY KEY(job_id,material_id),
 FOREIGN KEY(job_id,material_id) REFERENCES job_material_requirement(job_id,material_id) ON DELETE RESTRICT
);
CREATE INDEX material_reservation_active ON material_reservation(material_id,job_id) WHERE qty_reserved>0;
CREATE TABLE job_material_command (
 id uuid PRIMARY KEY,job_id uuid NOT NULL REFERENCES job(id) ON DELETE RESTRICT,
 actor_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,idempotency_key uuid NOT NULL,
 expected_version bigint NOT NULL CHECK(expected_version>=0),version bigint NOT NULL CHECK(version=expected_version+1),
 payload jsonb NOT NULL,result jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),UNIQUE(job_id,idempotency_key)
);

CREATE FUNCTION inventory_audit_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Inventory audit is append-only' USING ERRCODE='23514'; END $$;
CREATE TRIGGER stock_movement_immutable BEFORE UPDATE OR DELETE ON stock_movement FOR EACH ROW EXECUTE FUNCTION inventory_audit_immutable();
CREATE TRIGGER material_catalog_immutable BEFORE UPDATE OR DELETE ON material_catalog_audit FOR EACH ROW EXECUTE FUNCTION inventory_audit_immutable();
CREATE TRIGGER job_material_command_immutable BEFORE UPDATE OR DELETE ON job_material_command FOR EACH ROW EXECUTE FUNCTION inventory_audit_immutable();

CREATE FUNCTION protect_material_balance() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.unit_id<>OLD.unit_id THEN RAISE EXCEPTION 'Base unit is immutable' USING ERRCODE='23514'; END IF;
 IF (NEW.qty_on_hand,NEW.avg_cost,NEW.qty_reserved) IS DISTINCT FROM (OLD.qty_on_hand,OLD.avg_cost,OLD.qty_reserved) AND pg_trigger_depth()<2 THEN
  RAISE EXCEPTION 'Balance requires an audited movement/reservation' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER material_balance_guard BEFORE UPDATE ON material FOR EACH ROW EXECUTE FUNCTION protect_material_balance();

CREATE FUNCTION apply_stock_movement() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER stock_movement_apply BEFORE INSERT ON stock_movement FOR EACH ROW EXECUTE FUNCTION apply_stock_movement();

CREATE FUNCTION guard_material_requirement() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE j job; reserved numeric;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Release by setting requirement to zero' USING ERRCODE='23514';END IF;
 SELECT * INTO j FROM job WHERE id=NEW.job_id FOR UPDATE;
 SELECT qty_reserved INTO reserved FROM material_reservation WHERE job_id=NEW.job_id AND material_id=NEW.material_id;
 IF j.id IS NULL OR j.status<>'queued' OR COALESCE(reserved,0)>NEW.required_qty THEN
  RAISE EXCEPTION 'Job is not editable or still reserved' USING ERRCODE='23514';
 END IF;
 IF TG_OP='UPDATE' AND (OLD.job_id,OLD.material_id) IS DISTINCT FROM (NEW.job_id,NEW.material_id) THEN
  RAISE EXCEPTION 'Requirement identity is immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER material_requirement_guard BEFORE INSERT OR UPDATE OR DELETE ON job_material_requirement FOR EACH ROW EXECUTE FUNCTION guard_material_requirement();

CREATE FUNCTION apply_material_reservation() RETURNS trigger LANGUAGE plpgsql AS $$
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
CREATE TRIGGER material_reservation_apply AFTER INSERT OR UPDATE OR DELETE ON material_reservation FOR EACH ROW EXECUTE FUNCTION apply_material_reservation();
COMMENT ON TABLE stock_movement IS 'Admin additions/count corrections atomically update physical stock and append immutable snapshots; not supplier receiving';
COMMENT ON TABLE job_material_requirement IS 'Queued job plan, distinct from reservation and future actual usage; zero preserves removed-line identity';
COMMENT ON TABLE material_reservation IS 'Active/released reservations; triggers maintain material cached reserved quantity without consuming on-hand stock';
COMMENT ON TABLE job_material_command IS 'Actor, canonical command, optimistic version and result snapshot for audit/idempotent retries';
COMMENT ON COLUMN material.avg_cost IS 'Exact per-base-unit weighted average, NUMERIC(18,6), rounded half-up to six decimals on additions only';
