ALTER TABLE material_order ADD COLUMN credit_days integer CHECK(credit_days BETWEEN 0 AND 36500);
ALTER TABLE material_order ADD COLUMN full_received_at timestamptz;
ALTER TABLE material_order DROP CONSTRAINT material_order_status_check;
ALTER TABLE material_order ADD CONSTRAINT material_order_status_check CHECK(status IN ('ordered','partially_received','received','cancelled'));
ALTER TABLE material_order ADD CONSTRAINT material_order_full_receipt_check CHECK((status='received')=(full_received_at IS NOT NULL));
ALTER TABLE material_order_event DROP CONSTRAINT material_order_event_action_check;
ALTER TABLE material_order_event ADD CONSTRAINT material_order_event_action_check CHECK(action IN ('place','accept','reject','cancel','receive','claim','credit_terms','settle'));
CREATE TABLE material_receipt (
 id uuid PRIMARY KEY REFERENCES material_order_event(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 order_id uuid NOT NULL REFERENCES material_order(id) ON DELETE RESTRICT,
 actor_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 note text NOT NULL CHECK(length(note)<=2000),
 created_tx bigint NOT NULL DEFAULT txid_current(),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX material_receipt_order_idx ON material_receipt(order_id,created_at,id);
CREATE TABLE material_receipt_line (
 id uuid PRIMARY KEY,
 receipt_id uuid NOT NULL REFERENCES material_receipt(id) ON DELETE RESTRICT,
 order_id uuid NOT NULL,
 material_id uuid NOT NULL,
 good numeric(18,6) NOT NULL CHECK(good>=0 AND good<>'NaN'::numeric),
 damaged numeric(18,6) NOT NULL CHECK(damaged>=0 AND damaged<>'NaN'::numeric),
 replacement_claim_id uuid,
 CHECK(good+damaged>0),
 FOREIGN KEY(order_id,material_id) REFERENCES material_order_item(order_id,material_id) ON DELETE RESTRICT
);
CREATE UNIQUE INDEX material_receipt_line_unique ON material_receipt_line(receipt_id,material_id,COALESCE(replacement_claim_id,'00000000-0000-0000-0000-000000000000'));
CREATE INDEX material_receipt_line_order_idx ON material_receipt_line(order_id,material_id);
CREATE TABLE material_claim (
 id uuid PRIMARY KEY,
 source_line_id uuid NOT NULL UNIQUE REFERENCES material_receipt_line(id) ON DELETE RESTRICT,
 quantity numeric(18,6) NOT NULL CHECK(quantity>0 AND quantity<>'NaN'::numeric),
 replaced numeric(18,6) NOT NULL DEFAULT 0 CHECK(replaced>=0 AND replaced<=quantity AND replaced<>'NaN'::numeric),
 claimed_by uuid REFERENCES account(id) ON DELETE RESTRICT,
 claimed_at timestamptz,
 note text NOT NULL DEFAULT '' CHECK(length(note)<=2000),
 status text GENERATED ALWAYS AS (CASE WHEN replaced=quantity THEN 'resolved' WHEN claimed_at IS NOT NULL THEN 'claimed' ELSE 'pending_claim' END) STORED,
 CHECK((claimed_at IS NULL)=(claimed_by IS NULL))
);
ALTER TABLE material_receipt_line ADD CONSTRAINT material_receipt_replacement_fk FOREIGN KEY(replacement_claim_id) REFERENCES material_claim(id) ON DELETE RESTRICT;
CREATE INDEX material_receipt_replacement_idx ON material_receipt_line(replacement_claim_id) WHERE replacement_claim_id IS NOT NULL;
CREATE TABLE order_evidence (
 id uuid PRIMARY KEY,order_id uuid NOT NULL REFERENCES material_order(id) ON DELETE RESTRICT,
 uploaded_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 file_name text NOT NULL CHECK(length(file_name) BETWEEN 1 AND 200),
 content_type text NOT NULL CHECK(content_type IN ('application/pdf','application/msword','application/vnd.openxmlformats-officedocument.wordprocessingml.document','application/vnd.ms-excel','application/vnd.openxmlformats-officedocument.spreadsheetml.sheet','text/plain','image/jpeg','image/png')),
 byte_size bigint NOT NULL CHECK(byte_size BETWEEN 1 AND 100000000),
 staging_key text NOT NULL UNIQUE,object_key text NOT NULL UNIQUE,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','checking','ready','failed')),
 created_at timestamptz NOT NULL DEFAULT now(),expires_at timestamptz NOT NULL,completed_at timestamptz,
 CHECK(expires_at>created_at AND staging_key<>object_key),CHECK((state='ready')=(completed_at IS NOT NULL))
);
CREATE INDEX order_evidence_order_idx ON order_evidence(order_id,created_at,id);
CREATE TABLE supplier_settlement (
 order_id uuid PRIMARY KEY REFERENCES material_order(id) ON DELETE RESTRICT,
 event_id uuid NOT NULL UNIQUE REFERENCES material_order_event(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 actor_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 amount numeric(18,2) NOT NULL CHECK(amount>=0 AND amount<>'NaN'::numeric),
 method text NOT NULL CHECK(method IN ('bank','cash')),
 paid_at timestamptz NOT NULL,
 evidence_id uuid UNIQUE REFERENCES order_evidence(id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 CHECK(method<>'bank' OR evidence_id IS NOT NULL)
);
ALTER TABLE stock_movement ADD COLUMN receipt_line_id uuid UNIQUE REFERENCES material_receipt_line(id) ON DELETE RESTRICT;
DO $$ DECLARE c record; BEGIN
 FOR c IN SELECT conname FROM pg_constraint WHERE conrelid='stock_movement'::regclass AND contype='c' AND pg_get_constraintdef(oid) LIKE '%mode%' LOOP
  EXECUTE format('ALTER TABLE stock_movement DROP CONSTRAINT %I',c.conname);
 END LOOP;
END $$;
ALTER TABLE stock_movement ADD CONSTRAINT stock_movement_mode_check CHECK(mode IN ('addition','correction','purchase_receive'));
ALTER TABLE stock_movement ADD CONSTRAINT stock_movement_effect_check CHECK(
 (mode IN ('addition','purchase_receive') AND quantity>0 AND unit_cost IS NOT NULL) OR (mode='correction' AND unit_cost IS NULL));
ALTER TABLE stock_movement ADD CONSTRAINT stock_movement_receipt_check CHECK((mode='purchase_receive')=(receipt_line_id IS NOT NULL));

CREATE FUNCTION guard_material_receipt() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE o material_order;
BEGIN
 SELECT * INTO o FROM material_order WHERE id=NEW.order_id FOR UPDATE;
 IF o.supplier_response<>'accepted' OR o.status NOT IN ('ordered','partially_received') OR NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.actor_id AND is_active AND role IN ('admin','worker')) THEN
  RAISE EXCEPTION 'Order/receipt permission conflict' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER material_receipt_guard BEFORE INSERT ON material_receipt FOR EACH ROW EXECUTE FUNCTION guard_material_receipt();
CREATE FUNCTION guard_receipt_line() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE remaining numeric; parent_order uuid; c material_claim;
BEGIN
 SELECT order_id INTO parent_order FROM material_receipt WHERE id=NEW.receipt_id;
 IF NOT EXISTS(SELECT 1 FROM material_receipt WHERE id=NEW.receipt_id AND created_tx=txid_current()) THEN RAISE EXCEPTION 'Receipt is already finalized' USING ERRCODE='23514'; END IF;
 IF parent_order IS DISTINCT FROM NEW.order_id THEN RAISE EXCEPTION 'Receipt order mismatch' USING ERRCODE='23514'; END IF;
 IF NEW.replacement_claim_id IS NULL THEN
  SELECT i.qty_ordered-COALESCE((SELECT sum(good+damaged) FROM material_receipt_line l WHERE l.order_id=NEW.order_id AND l.material_id=NEW.material_id AND replacement_claim_id IS NULL),0)
   INTO remaining FROM material_order_item i WHERE i.order_id=NEW.order_id AND i.material_id=NEW.material_id;
 ELSE
  SELECT cl.* INTO c FROM material_claim cl JOIN material_receipt_line src ON src.id=cl.source_line_id
   WHERE cl.id=NEW.replacement_claim_id AND src.order_id=NEW.order_id AND src.material_id=NEW.material_id;
  IF c.id IS NULL OR c.status<>'claimed' THEN RAISE EXCEPTION 'Replacement requires an open claimed defect' USING ERRCODE='23514'; END IF;
  SELECT c.quantity-COALESCE(sum(good),0) INTO remaining FROM material_receipt_line WHERE replacement_claim_id=c.id;
 END IF;
 IF remaining IS NULL OR NEW.good+NEW.damaged>remaining THEN RAISE EXCEPTION 'Excess delivery' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER material_receipt_line_guard BEFORE INSERT ON material_receipt_line FOR EACH ROW EXECUTE FUNCTION guard_receipt_line();
CREATE FUNCTION guard_material_claim() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE src material_receipt_line; replaced_qty numeric;
BEGIN
 SELECT * INTO src FROM material_receipt_line WHERE id=NEW.source_line_id;
 IF src.replacement_claim_id IS NOT NULL OR NEW.quantity<>src.damaged THEN RAISE EXCEPTION 'Claim must match original damaged delivery' USING ERRCODE='23514'; END IF;
 SELECT COALESCE(sum(good),0) INTO replaced_qty FROM material_receipt_line WHERE replacement_claim_id=NEW.id;
 IF NEW.replaced<>replaced_qty THEN RAISE EXCEPTION 'Claim replacement total mismatch' USING ERRCODE='23514'; END IF;
 IF TG_OP='UPDATE' THEN
  IF (NEW.id,NEW.source_line_id,NEW.quantity) IS DISTINCT FROM (OLD.id,OLD.source_line_id,OLD.quantity)
   OR NEW.replaced<OLD.replaced OR (OLD.claimed_at IS NOT NULL AND (NEW.claimed_at,NEW.claimed_by,NEW.note) IS DISTINCT FROM (OLD.claimed_at,OLD.claimed_by,OLD.note)) THEN
   RAISE EXCEPTION 'Claim history is immutable' USING ERRCODE='23514';
  END IF;
 END IF;
 IF NEW.claimed_by IS NOT NULL AND NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.claimed_by AND is_active AND role IN ('admin','worker')) THEN RAISE EXCEPTION 'Claim staff required' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER material_claim_guard BEFORE INSERT OR UPDATE ON material_claim FOR EACH ROW EXECUTE FUNCTION guard_material_claim();
CREATE TRIGGER material_claim_no_delete BEFORE DELETE ON material_claim FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE TRIGGER material_receipt_immutable BEFORE UPDATE OR DELETE ON material_receipt FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE TRIGGER material_receipt_line_immutable BEFORE UPDATE OR DELETE ON material_receipt_line FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE TRIGGER supplier_settlement_immutable BEFORE UPDATE OR DELETE ON supplier_settlement FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE FUNCTION guard_order_evidence() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' OR OLD.state IN ('ready','failed') OR (to_jsonb(NEW)-'state'-'completed_at') IS DISTINCT FROM (to_jsonb(OLD)-'state'-'completed_at')
 OR NOT ((OLD.state='pending' AND NEW.state='checking') OR (OLD.state='checking' AND NEW.state IN ('ready','failed'))) THEN
  RAISE EXCEPTION 'Evidence metadata/state is immutable' USING ERRCODE='23514';
 END IF;RETURN NEW;
END $$;
CREATE TRIGGER order_evidence_guard BEFORE UPDATE OR DELETE ON order_evidence FOR EACH ROW EXECUTE FUNCTION guard_order_evidence();
CREATE FUNCTION guard_supplier_settlement() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE o material_order;
BEGIN
 SELECT * INTO o FROM material_order WHERE id=NEW.order_id FOR UPDATE;
 IF o.status<>'received' OR o.credit_days IS NULL OR NEW.amount<>o.total_amount OR NEW.paid_at<o.full_received_at OR NEW.paid_at>clock_timestamp()
 OR NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.actor_id AND is_active AND role='admin') THEN RAISE EXCEPTION 'Full receipt/terms/Admin settlement required' USING ERRCODE='23514'; END IF;
 IF NEW.evidence_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM order_evidence WHERE id=NEW.evidence_id AND order_id=o.id AND state='ready') THEN RAISE EXCEPTION 'Ready order evidence required' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER supplier_settlement_guard BEFORE INSERT ON supplier_settlement FOR EACH ROW EXECUTE FUNCTION guard_supplier_settlement();

CREATE OR REPLACE FUNCTION apply_stock_movement() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE m material; next_qty numeric; next_cost numeric;
BEGIN
 SELECT * INTO m FROM material WHERE id=NEW.material_id FOR UPDATE;
 IF m.id IS NULL OR m.version<>NEW.expected_version OR NEW.before_quantity<>m.qty_on_hand OR NEW.before_cost<>m.avg_cost
 OR NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.actor_id AND is_active AND (role='admin' OR (NEW.mode='purchase_receive' AND role='worker'))) THEN
  RAISE EXCEPTION 'Stock snapshot/permission conflict' USING ERRCODE='23514'; END IF;
 IF NEW.mode IN ('addition','purchase_receive') THEN
  IF NOT m.is_active THEN RAISE EXCEPTION 'Material is inactive' USING ERRCODE='23514'; END IF;
  IF NEW.mode='purchase_receive' AND NOT EXISTS(SELECT 1 FROM material_receipt_line l JOIN material_receipt r ON r.id=l.receipt_id JOIN material_order_item i ON (i.order_id,i.material_id)=(l.order_id,l.material_id)
   WHERE l.id=NEW.receipt_line_id AND l.material_id=m.id AND l.good=NEW.quantity AND i.unit_cost=NEW.unit_cost AND r.actor_id=NEW.actor_id) THEN
   RAISE EXCEPTION 'Receipt ledger mismatch' USING ERRCODE='23514'; END IF;
  next_qty:=m.qty_on_hand+NEW.quantity;next_cost:=round((m.qty_on_hand*m.avg_cost+NEW.quantity*NEW.unit_cost)/next_qty,6);
 ELSE next_qty:=NEW.quantity;next_cost:=m.avg_cost;END IF;
 IF NEW.after_quantity<>next_qty OR NEW.after_cost<>next_cost OR next_qty<m.qty_reserved THEN RAISE EXCEPTION 'Invalid stock effect' USING ERRCODE='23514'; END IF;
 UPDATE material SET qty_on_hand=next_qty,avg_cost=next_cost,version=NEW.version,edited_at=clock_timestamp() WHERE id=m.id;RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION procurement_order_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Orders cannot be deleted' USING ERRCODE='23514'; END IF;
 IF (to_jsonb(NEW)-'status'-'supplier_response'-'version'-'credit_days'-'full_received_at') IS DISTINCT FROM (to_jsonb(OLD)-'status'-'supplier_response'-'version'-'credit_days'-'full_received_at')
 OR NEW.version<>OLD.version+1 OR OLD.status='cancelled' OR (OLD.credit_days IS NOT NULL AND NEW.credit_days IS DISTINCT FROM OLD.credit_days)
 OR (OLD.full_received_at IS NOT NULL AND NEW.full_received_at IS DISTINCT FROM OLD.full_received_at) THEN RAISE EXCEPTION 'Invalid order edit' USING ERRCODE='23514'; END IF;
 IF NEW.supplier_response IS DISTINCT FROM OLD.supplier_response AND NOT (OLD.status='ordered' AND OLD.supplier_response='pending' AND ((NEW.supplier_response='accepted' AND NEW.status='ordered') OR (NEW.supplier_response='rejected' AND NEW.status='cancelled'))) THEN RAISE EXCEPTION 'Invalid supplier response' USING ERRCODE='23514'; END IF;
 IF NEW.status='cancelled' AND EXISTS(SELECT 1 FROM material_receipt WHERE order_id=OLD.id) THEN RAISE EXCEPTION 'Cannot cancel after any receipt' USING ERRCODE='23514'; END IF;
 IF (NEW.status IN ('partially_received','received') AND NEW.supplier_response<>'accepted') OR (OLD.status='received' AND NEW.status<>'received') OR (OLD.status='partially_received' AND NEW.status NOT IN ('partially_received','received')) THEN RAISE EXCEPTION 'Invalid receipt transition' USING ERRCODE='23514'; END IF;
 RETURN NEW;
END $$;
CREATE OR REPLACE FUNCTION procurement_order_check() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE v material_order; line_count int; line_sum numeric; event_action text; full_good boolean;
BEGIN
 SELECT * INTO v FROM material_order WHERE id=NEW.id;
 SELECT count(*),sum(total_amount) INTO line_count,line_sum FROM material_order_item WHERE order_id=v.id;
 IF line_count NOT BETWEEN 1 AND 100 OR line_sum IS DISTINCT FROM v.total_amount THEN RAISE EXCEPTION 'Order total mismatch' USING ERRCODE='23514'; END IF;
 SELECT action INTO event_action FROM material_order_event WHERE order_id=v.id AND version=v.version;
 IF event_action IS NULL OR (v.version=1 AND event_action<>'place') THEN RAISE EXCEPTION 'Order audit required' USING ERRCODE='23514'; END IF;
 IF event_action='receive' THEN
  SELECT bool_and(i.qty_ordered=COALESCE((SELECT sum(good) FROM material_receipt_line l WHERE l.order_id=i.order_id AND l.material_id=i.material_id),0)) INTO full_good FROM material_order_item i WHERE i.order_id=v.id;
  IF v.status<>CASE WHEN full_good THEN 'received' ELSE 'partially_received' END THEN RAISE EXCEPTION 'Receipt progress mismatch' USING ERRCODE='23514'; END IF;
 ELSIF event_action IN ('claim','credit_terms','settle') THEN
  IF event_action='settle' AND NOT EXISTS(SELECT 1 FROM supplier_settlement WHERE order_id=v.id) THEN RAISE EXCEPTION 'Settlement missing' USING ERRCODE='23514'; END IF;
 ELSIF v.version>1 AND ((v.status='ordered' AND event_action<>'accept') OR (v.status='cancelled' AND v.supplier_response='rejected' AND event_action<>'reject') OR (v.status='cancelled' AND v.supplier_response<>'rejected' AND event_action<>'cancel')) THEN RAISE EXCEPTION 'Order event mismatch' USING ERRCODE='23514'; END IF;
 RETURN NULL;
END $$;
CREATE FUNCTION check_material_receipt() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM material_order_event e JOIN material_order o ON o.id=e.order_id WHERE e.id=NEW.id AND e.order_id=NEW.order_id AND e.action='receive' AND e.version=o.version) THEN
  RAISE EXCEPTION 'Receipt requires its current order event' USING ERRCODE='23514'; END IF;
 IF EXISTS(SELECT 1 FROM material_claim c JOIN material_receipt_line src ON src.id=c.source_line_id WHERE src.order_id=NEW.order_id AND c.replaced<>(SELECT COALESCE(sum(l.good),0) FROM material_receipt_line l WHERE l.replacement_claim_id=c.id)) THEN
  RAISE EXCEPTION 'Claim totals must reconcile with good replacements' USING ERRCODE='23514'; END IF;
 IF NOT EXISTS(SELECT 1 FROM material_receipt_line WHERE receipt_id=NEW.id) OR EXISTS(
  SELECT 1 FROM material_receipt_line l WHERE l.receipt_id=NEW.id AND
  ((l.good>0 AND NOT EXISTS(SELECT 1 FROM stock_movement s WHERE s.receipt_line_id=l.id))
   OR (l.replacement_claim_id IS NULL AND l.damaged>0 AND NOT EXISTS(SELECT 1 FROM material_claim c WHERE c.source_line_id=l.id AND c.quantity=l.damaged)))) THEN
  RAISE EXCEPTION 'Receipt requires complete ledger/defect audit' USING ERRCODE='23514'; END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER material_receipt_consistent AFTER INSERT ON material_receipt DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION check_material_receipt();
COMMENT ON TABLE material_claim IS 'Replacement-only; original damaged units stay outstanding. Bad replacement attempts do not create a second claim.';
COMMENT ON TABLE order_evidence IS 'Private Admin-only supplier evidence; never customer/request attachments or public URLs.';
COMMENT ON COLUMN material_order.credit_days IS 'Admin enters once, no guessed default; due = full_received_at plus this many 24-hour days.';
