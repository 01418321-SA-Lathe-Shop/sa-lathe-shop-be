ALTER TABLE job ADD COLUMN production_version bigint NOT NULL DEFAULT 0 CHECK(production_version>=0),
 ADD COLUMN started_at timestamptz,ADD COLUMN completed_at timestamptz,
 ADD COLUMN actual_labor_cost numeric(18,2) CHECK(actual_labor_cost>=0 AND actual_labor_cost<>'NaN'::numeric);
ALTER TABLE material_reservation ADD COLUMN qty_consumed numeric(18,6) NOT NULL DEFAULT 0 CHECK(qty_consumed>=0 AND qty_consumed<>'NaN'::numeric);
ALTER TABLE material_reservation DROP COLUMN status;
ALTER TABLE material_reservation ADD COLUMN status text GENERATED ALWAYS AS (CASE WHEN qty_reserved>0 THEN 'reserved' WHEN qty_consumed>0 THEN 'consumed' ELSE 'released' END) STORED;
CREATE TABLE production_event (
 id uuid PRIMARY KEY,job_id uuid NOT NULL REFERENCES job(id) ON DELETE RESTRICT,
 actor_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 action text NOT NULL CHECK(action IN ('start','use','time','complete','labor')),
 version bigint NOT NULL CHECK(version>0),command jsonb NOT NULL,result jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),UNIQUE(job_id,version)
);
CREATE TABLE job_material_usage (
 id uuid PRIMARY KEY,event_id uuid NOT NULL REFERENCES production_event(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 job_id uuid NOT NULL REFERENCES job(id) ON DELETE RESTRICT,material_id uuid NOT NULL REFERENCES material(id) ON DELETE RESTRICT,
 actor_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,material_name text NOT NULL,unit_name text NOT NULL,
 quantity numeric(18,6) NOT NULL CHECK(quantity>0 AND quantity<>'NaN'::numeric),
 reservation_used numeric(18,6) NOT NULL CHECK(reservation_used>=0 AND reservation_used<=quantity AND reservation_used<>'NaN'::numeric),
 unit_cost numeric(18,6) NOT NULL CHECK(unit_cost>=0 AND unit_cost<>'NaN'::numeric),
 total_amount numeric(18,2) GENERATED ALWAYS AS (round(quantity*unit_cost,2)) STORED,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),UNIQUE(event_id,material_id)
);
CREATE INDEX job_material_usage_job_idx ON job_material_usage(job_id,created_at,id);
CREATE INDEX job_material_usage_material_idx ON job_material_usage(material_id,job_id);
CREATE TABLE job_time_entry (
 id uuid PRIMARY KEY REFERENCES production_event(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 job_id uuid NOT NULL REFERENCES job(id) ON DELETE RESTRICT,actor_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 minutes bigint NOT NULL CHECK(minutes BETWEEN 1 AND 1000000),note text NOT NULL CHECK(length(note)<=2000),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX job_time_entry_job_idx ON job_time_entry(job_id,created_at,id);
CREATE TABLE job_photo (
 id uuid PRIMARY KEY,job_id uuid NOT NULL REFERENCES job(id) ON DELETE RESTRICT,uploaded_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 file_name text NOT NULL CHECK(length(file_name) BETWEEN 1 AND 200),content_type text NOT NULL CHECK(content_type IN ('image/jpeg','image/png')),
 byte_size bigint NOT NULL CHECK(byte_size BETWEEN 1 AND 100000000),staging_key text NOT NULL UNIQUE,object_key text NOT NULL UNIQUE,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','checking','ready','failed')),
 created_at timestamptz NOT NULL DEFAULT now(),expires_at timestamptz NOT NULL,completed_at timestamptz,
 CHECK(expires_at>created_at AND staging_key<>object_key),CHECK((state='ready')=(completed_at IS NOT NULL))
);
CREATE INDEX job_photo_job_idx ON job_photo(job_id,created_at,id);
CREATE TRIGGER production_event_immutable BEFORE UPDATE OR DELETE ON production_event FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE TRIGGER job_material_usage_immutable BEFORE UPDATE OR DELETE ON job_material_usage FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE TRIGGER job_time_entry_immutable BEFORE UPDATE OR DELETE ON job_time_entry FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE TRIGGER job_photo_guard BEFORE UPDATE OR DELETE ON job_photo FOR EACH ROW EXECUTE FUNCTION guard_order_evidence();
ALTER TABLE stock_movement ADD COLUMN job_usage_id uuid UNIQUE REFERENCES job_material_usage(id) ON DELETE RESTRICT;
ALTER TABLE stock_movement DROP CONSTRAINT stock_movement_mode_check,DROP CONSTRAINT stock_movement_effect_check;
ALTER TABLE stock_movement ADD CONSTRAINT stock_movement_mode_check CHECK(mode IN ('addition','correction','purchase_receive','job_issue'));
ALTER TABLE stock_movement ADD CONSTRAINT stock_movement_effect_check CHECK((mode IN ('addition','purchase_receive','job_issue') AND quantity>0 AND unit_cost IS NOT NULL) OR (mode='correction' AND unit_cost IS NULL));
ALTER TABLE stock_movement ADD CONSTRAINT stock_movement_usage_check CHECK((mode='job_issue')=(job_usage_id IS NOT NULL));

CREATE OR REPLACE FUNCTION apply_material_reservation() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE needed numeric;previous numeric:=0;consumed numeric:=0;m material;j job;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Release reservations explicitly' USING ERRCODE='23514'; END IF;
 IF TG_OP='UPDATE' THEN
  IF (OLD.job_id,OLD.material_id) IS DISTINCT FROM (NEW.job_id,NEW.material_id) THEN RAISE EXCEPTION 'Reservation identity is immutable' USING ERRCODE='23514'; END IF;
  previous:=OLD.qty_reserved;consumed:=OLD.qty_consumed;
 END IF;
 SELECT * INTO j FROM job WHERE id=NEW.job_id FOR UPDATE;
 SELECT * INTO m FROM material WHERE id=NEW.material_id FOR UPDATE;
 SELECT required_qty INTO needed FROM job_material_requirement WHERE job_id=NEW.job_id AND material_id=NEW.material_id;
 IF needed IS NULL OR NEW.qty_reserved>needed THEN RAISE EXCEPTION 'Invalid requirement reservation' USING ERRCODE='23514'; END IF;
 IF j.status='queued' THEN
  IF NEW.qty_consumed<>consumed OR (NOT m.is_active AND NEW.qty_reserved>previous) THEN RAISE EXCEPTION 'Invalid queued allocation' USING ERRCODE='23514'; END IF;
 ELSIF j.status='in_progress' THEN
  IF TG_OP<>'UPDATE' OR NEW.qty_reserved>previous OR NEW.qty_consumed<consumed OR NEW.qty_consumed-consumed>previous-NEW.qty_reserved THEN RAISE EXCEPTION 'Production may only consume/release own reservation' USING ERRCODE='23514'; END IF;
 ELSE RAISE EXCEPTION 'Job reservation is not editable' USING ERRCODE='23514'; END IF;
 UPDATE material SET qty_reserved=qty_reserved+NEW.qty_reserved-previous,version=version+CASE WHEN NEW.qty_reserved<>previous THEN 1 ELSE 0 END,edited_at=clock_timestamp() WHERE id=m.id;RETURN NEW;
END $$;
CREATE FUNCTION guard_production_usage() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE j job;m material;own_qty numeric;used_qty numeric;
BEGIN
 SELECT * INTO j FROM job WHERE id=NEW.job_id FOR UPDATE;
 SELECT * INTO m FROM material WHERE id=NEW.material_id FOR UPDATE;
 SELECT qty_reserved INTO own_qty FROM material_reservation WHERE job_id=j.id AND material_id=m.id;own_qty:=COALESCE(own_qty,0);used_qty:=LEAST(own_qty,NEW.quantity);
 IF j.status<>'in_progress' OR NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.actor_id AND is_active AND role='worker')
 OR NEW.reservation_used<>used_qty OR NEW.quantity-used_qty>m.qty_on_hand-m.qty_reserved OR NEW.unit_cost<>m.avg_cost
 OR NEW.material_name<>m.name OR NEW.unit_name<>(SELECT name FROM unit WHERE id=m.unit_id) OR (NOT m.is_active AND NEW.quantity>used_qty) THEN
  RAISE EXCEPTION 'Invalid production issue/valuation' USING ERRCODE='23514'; END IF;
 IF used_qty>0 THEN UPDATE material_reservation SET qty_reserved=qty_reserved-used_qty,qty_consumed=qty_consumed+used_qty WHERE job_id=j.id AND material_id=m.id; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER production_usage_guard BEFORE INSERT ON job_material_usage FOR EACH ROW EXECUTE FUNCTION guard_production_usage();
CREATE FUNCTION check_production_usage() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM stock_movement WHERE job_usage_id=NEW.id) OR NOT EXISTS(SELECT 1 FROM production_event WHERE id=NEW.event_id AND job_id=NEW.job_id AND actor_id=NEW.actor_id AND action='use') THEN RAISE EXCEPTION 'Usage requires matching ledger/event' USING ERRCODE='23514'; END IF; RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER production_usage_consistent AFTER INSERT ON job_material_usage DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_production_usage();
CREATE FUNCTION check_consumed_reservation() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE consumed numeric;actual numeric;
BEGIN
 SELECT qty_consumed INTO consumed FROM material_reservation WHERE job_id=NEW.job_id AND material_id=NEW.material_id;
 SELECT COALESCE(sum(reservation_used),0) INTO actual FROM job_material_usage WHERE job_id=NEW.job_id AND material_id=NEW.material_id;
 IF consumed<>actual THEN RAISE EXCEPTION 'Consumed reservation must match actual usage' USING ERRCODE='23514'; END IF;RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER consumed_reservation_consistent AFTER INSERT OR UPDATE ON material_reservation DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_consumed_reservation();
CREATE FUNCTION guard_production_job() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF (NEW.status,NEW.production_version,NEW.started_at,NEW.completed_at,NEW.actual_labor_cost) IS NOT DISTINCT FROM (OLD.status,OLD.production_version,OLD.started_at,OLD.completed_at,OLD.actual_labor_cost) THEN RETURN NEW;END IF;
 IF NEW.production_version<>OLD.production_version+1 OR (OLD.started_at IS NOT NULL AND NEW.started_at IS DISTINCT FROM OLD.started_at) OR (OLD.completed_at IS NOT NULL AND NEW.completed_at IS DISTINCT FROM OLD.completed_at) THEN RAISE EXCEPTION 'Production requires versioned immutable timestamps' USING ERRCODE='23514'; END IF;
 IF OLD.status='queued' AND NEW.status='in_progress' THEN
  IF NEW.started_at IS NULL OR NEW.completed_at IS NOT NULL OR NOT EXISTS(SELECT 1 FROM job_material_requirement WHERE job_id=NEW.id AND required_qty>0)
   OR EXISTS(SELECT 1 FROM job_material_requirement q LEFT JOIN material_reservation r USING(job_id,material_id) WHERE q.job_id=NEW.id AND q.required_qty>COALESCE(r.qty_reserved,0)) THEN RAISE EXCEPTION 'Fully reserved nonempty plan required' USING ERRCODE='23514'; END IF;
 ELSIF OLD.status='in_progress' AND NEW.status='completed' THEN
  IF NEW.completed_at IS NULL OR NEW.completed_at<NEW.started_at OR EXISTS(SELECT 1 FROM material_reservation WHERE job_id=NEW.id AND qty_reserved>0) THEN RAISE EXCEPTION 'Completion must release leftover reservations' USING ERRCODE='23514'; END IF;
 ELSIF OLD.status IN ('in_progress','completed') AND NEW.status=OLD.status THEN NULL;
 ELSE RAISE EXCEPTION 'Unsupported production transition' USING ERRCODE='23514';END IF;RETURN NEW;
END $$;
CREATE TRIGGER production_job_guard BEFORE UPDATE ON job FOR EACH ROW EXECUTE FUNCTION guard_production_job();
CREATE FUNCTION check_production_job() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE j job;e production_event;
BEGIN
 SELECT * INTO j FROM job WHERE id=NEW.id;
 IF j.production_version=0 THEN RETURN NULL;END IF;
 SELECT * INTO e FROM production_event WHERE job_id=j.id AND version=j.production_version;
 IF e.id IS NULL OR NOT EXISTS(SELECT 1 FROM account WHERE id=e.actor_id AND ((e.action='labor' AND role='admin') OR (e.action<>'labor' AND role='worker'))) THEN RAISE EXCEPTION 'Production audit/role required' USING ERRCODE='23514';END IF;
 IF (e.action='start' AND j.status<>'in_progress') OR (e.action='complete' AND j.status<>'completed') OR (e.action IN ('use','time') AND j.status<>'in_progress') THEN RAISE EXCEPTION 'Production status/event mismatch' USING ERRCODE='23514';END IF;
 IF e.action='time' AND NOT EXISTS(SELECT 1 FROM job_time_entry WHERE id=e.id AND job_id=j.id AND actor_id=e.actor_id) THEN RAISE EXCEPTION 'Time entry required' USING ERRCODE='23514';END IF;
 IF e.action='use' AND NOT EXISTS(SELECT 1 FROM job_material_usage WHERE event_id=e.id AND job_id=j.id) THEN RAISE EXCEPTION 'Usage required' USING ERRCODE='23514';END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER production_job_consistent AFTER UPDATE ON job DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_production_job();

CREATE OR REPLACE FUNCTION apply_stock_movement() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE m material;next_qty numeric;next_cost numeric;
BEGIN
 SELECT * INTO m FROM material WHERE id=NEW.material_id FOR UPDATE;
 IF m.id IS NULL OR m.version<>NEW.expected_version OR NEW.before_quantity<>m.qty_on_hand OR NEW.before_cost<>m.avg_cost OR NOT EXISTS(
 SELECT 1 FROM account WHERE id=NEW.actor_id AND is_active AND ((NEW.mode='job_issue' AND role='worker') OR (NEW.mode<>'job_issue' AND (role='admin' OR (NEW.mode='purchase_receive' AND role='worker'))))) THEN RAISE EXCEPTION 'Stock snapshot/permission conflict' USING ERRCODE='23514';END IF;
 IF NEW.mode IN ('addition','purchase_receive') THEN
  IF NOT m.is_active THEN RAISE EXCEPTION 'Material is inactive' USING ERRCODE='23514'; END IF;
  IF NEW.mode='purchase_receive' AND NOT EXISTS(SELECT 1 FROM material_receipt_line l JOIN material_receipt r ON r.id=l.receipt_id JOIN material_order_item i ON (i.order_id,i.material_id)=(l.order_id,l.material_id) WHERE l.id=NEW.receipt_line_id AND l.material_id=m.id AND l.good=NEW.quantity AND i.unit_cost=NEW.unit_cost AND r.actor_id=NEW.actor_id) THEN RAISE EXCEPTION 'Receipt ledger mismatch' USING ERRCODE='23514';END IF;
  next_qty:=m.qty_on_hand+NEW.quantity;next_cost:=round((m.qty_on_hand*m.avg_cost+NEW.quantity*NEW.unit_cost)/next_qty,6);
 ELSIF NEW.mode='job_issue' THEN
  IF NOT EXISTS(SELECT 1 FROM job_material_usage u JOIN job j ON j.id=u.job_id WHERE u.id=NEW.job_usage_id AND u.material_id=m.id AND u.actor_id=NEW.actor_id AND u.quantity=NEW.quantity AND u.unit_cost=NEW.unit_cost AND j.status='in_progress') THEN RAISE EXCEPTION 'Usage ledger mismatch' USING ERRCODE='23514';END IF;
  next_qty:=m.qty_on_hand-NEW.quantity;next_cost:=m.avg_cost;
 ELSE next_qty:=NEW.quantity;next_cost:=m.avg_cost;END IF;
 IF NEW.after_quantity<>next_qty OR NEW.after_cost<>next_cost OR next_qty<m.qty_reserved THEN RAISE EXCEPTION 'Invalid stock effect' USING ERRCODE='23514';END IF;
 UPDATE material SET qty_on_hand=next_qty,avg_cost=next_cost,version=NEW.version,edited_at=clock_timestamp() WHERE id=m.id;RETURN NEW;
END $$;
COMMENT ON TABLE job_material_usage IS 'Immutable actual incremental issues; own reservation first, then free stock. Snapshot price and half-up per-issue THB valuation.';
COMMENT ON TABLE job_photo IS 'Worker-uploaded private JPEG/PNG objects, staff-readable; never customer/payment attachments.';
COMMENT ON COLUMN job.actual_labor_cost IS 'Admin-entered actual labor total; versioned updates audited, never inferred from minutes or a guessed rate.';
