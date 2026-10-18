CREATE TABLE supplier (
 id uuid PRIMARY KEY,
 name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
 email text NOT NULL DEFAULT '' CHECK (length(email)<=254),
 phone text NOT NULL DEFAULT '' CHECK (phone='' OR phone ~ '^\+?[0-9]{7,15}$'),
 is_active boolean NOT NULL DEFAULT true,
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 created_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT now(),
 edited_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX supplier_name_idx ON supplier(lower(name),id);
CREATE TABLE supplier_audit (
 id uuid PRIMARY KEY,
 supplier_id uuid NOT NULL REFERENCES supplier(id) ON DELETE RESTRICT,
 actor_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 before_value jsonb,
 after_value jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX supplier_audit_supplier_idx ON supplier_audit(supplier_id,created_at,id);
CREATE TABLE material_order (
 id uuid PRIMARY KEY,
 order_no text NOT NULL UNIQUE,
 supplier_id uuid NOT NULL REFERENCES supplier(id) ON DELETE RESTRICT,
 supplier_snapshot jsonb NOT NULL CHECK(jsonb_typeof(supplier_snapshot)='object'),
 status text NOT NULL DEFAULT 'ordered' CHECK(status IN ('ordered','cancelled')),
 supplier_response text NOT NULL DEFAULT 'pending' CHECK(supplier_response IN ('pending','accepted','rejected')),
 total_amount numeric(18,2) NOT NULL CHECK(total_amount>=0 AND total_amount<>'NaN'::numeric),
 version bigint NOT NULL DEFAULT 1 CHECK(version>0),
 expected_at timestamptz,
 created_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT now(),
 CHECK(supplier_response<>'rejected' OR status='cancelled')
);
CREATE INDEX material_order_created_idx ON material_order(created_at DESC,id DESC);
CREATE INDEX material_order_supplier_idx ON material_order(supplier_id,created_at DESC);
-- Job links are references, NOT allocations or incoming-stock coverage.
CREATE TABLE material_order_job (
 order_id uuid NOT NULL REFERENCES material_order(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 job_id uuid NOT NULL REFERENCES job(id) ON DELETE RESTRICT,
 PRIMARY KEY(order_id,job_id)
);
CREATE INDEX material_order_job_job_idx ON material_order_job(job_id,order_id);
CREATE TABLE material_order_item (
 order_id uuid NOT NULL REFERENCES material_order(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 material_id uuid NOT NULL REFERENCES material(id) ON DELETE RESTRICT,
 material_name text NOT NULL,
 unit_id uuid NOT NULL REFERENCES unit(id) ON DELETE RESTRICT,
 unit_name text NOT NULL,
 qty_ordered numeric(18,6) NOT NULL CHECK(qty_ordered>0 AND qty_ordered<>'NaN'::numeric),
 unit_cost numeric(18,6) NOT NULL CHECK(unit_cost>=0 AND unit_cost<>'NaN'::numeric),
 total_amount numeric(18,2) GENERATED ALWAYS AS (round(qty_ordered*unit_cost,2)) STORED,
 PRIMARY KEY(order_id,material_id)
);
CREATE INDEX material_order_item_material_idx ON material_order_item(material_id,order_id);
CREATE TABLE material_order_event (
 id uuid PRIMARY KEY, -- globally unique idempotency key
 order_id uuid NOT NULL REFERENCES material_order(id) ON DELETE RESTRICT DEFERRABLE INITIALLY DEFERRED,
 actor_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 action text NOT NULL CHECK(action IN ('place','accept','reject','cancel')),
 reason text NOT NULL DEFAULT '' CHECK(length(reason)<=2000 AND (action NOT IN ('reject','cancel') OR length(btrim(reason))>0)),
 version bigint NOT NULL CHECK(version>0),
 command jsonb NOT NULL,
 result jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(order_id,version)
);
COMMENT ON TABLE material_order_job IS 'Flexible many-job references; no quantities, shortage caps or UC5 plan locks.';
COMMENT ON TABLE material_order_event IS 'Immutable command, actor and result snapshots support exact retries without duplicate supplier commitments.';
COMMENT ON COLUMN material_order.total_amount IS 'THB, sum of half-up two-decimal line totals; no VAT. Not stock value.';
CREATE FUNCTION procurement_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'Procurement history is immutable' USING ERRCODE='23514'; END $$;
CREATE TRIGGER supplier_audit_immutable BEFORE UPDATE OR DELETE ON supplier_audit FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE TRIGGER material_order_event_immutable BEFORE UPDATE OR DELETE ON material_order_event FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE TRIGGER material_order_item_immutable BEFORE UPDATE OR DELETE ON material_order_item FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
CREATE TRIGGER material_order_job_immutable BEFORE UPDATE OR DELETE ON material_order_job FOR EACH ROW EXECUTE FUNCTION procurement_immutable();
-- Insert children first and their parent last in the placement transaction.
CREATE FUNCTION procurement_child_insert() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM material_order WHERE id=NEW.order_id) THEN
  RAISE EXCEPTION 'Placed order lines and references are immutable' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER material_order_item_insert BEFORE INSERT ON material_order_item FOR EACH ROW EXECUTE FUNCTION procurement_child_insert();
CREATE TRIGGER material_order_job_insert BEFORE INSERT ON material_order_job FOR EACH ROW EXECUTE FUNCTION procurement_child_insert();
CREATE FUNCTION procurement_order_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Orders cannot be deleted' USING ERRCODE='23514'; END IF;
 IF (to_jsonb(NEW)-'status'-'supplier_response'-'version') IS DISTINCT FROM (to_jsonb(OLD)-'status'-'supplier_response'-'version')
  OR NEW.version<>OLD.version+1 OR OLD.status<>'ordered' THEN
  RAISE EXCEPTION 'Invalid or non-versioned order edit' USING ERRCODE='23514';
 END IF;
 IF NOT ((OLD.supplier_response='pending' AND NEW.supplier_response='accepted' AND NEW.status='ordered')
  OR (OLD.supplier_response='pending' AND NEW.supplier_response='rejected' AND NEW.status='cancelled')
  OR (NEW.supplier_response=OLD.supplier_response AND NEW.status='cancelled')) THEN
  RAISE EXCEPTION 'Invalid supplier order transition' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER material_order_guard BEFORE UPDATE OR DELETE ON material_order FOR EACH ROW EXECUTE FUNCTION procurement_order_guard();
CREATE FUNCTION procurement_order_check() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE v material_order; line_count int; line_sum numeric; event_action text;
BEGIN
 SELECT * INTO v FROM material_order WHERE id=NEW.id;
 SELECT count(*),sum(total_amount) INTO line_count,line_sum FROM material_order_item WHERE order_id=v.id;
 IF line_count NOT BETWEEN 1 AND 100 OR line_sum IS DISTINCT FROM v.total_amount THEN
  RAISE EXCEPTION 'Order total must equal rounded item totals' USING ERRCODE='23514';
 END IF;
 SELECT action INTO event_action FROM material_order_event WHERE order_id=v.id AND version=v.version;
 IF event_action IS NULL OR (v.version=1 AND event_action<>'place')
  OR (v.version>1 AND ((v.status='ordered' AND event_action<>'accept')
   OR (v.status='cancelled' AND v.supplier_response='rejected' AND event_action<>'reject')
   OR (v.status='cancelled' AND v.supplier_response<>'rejected' AND event_action<>'cancel'))) THEN
  RAISE EXCEPTION 'Order transition requires matching audit event' USING ERRCODE='23514';
 END IF;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER material_order_consistent AFTER INSERT OR UPDATE ON material_order DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION procurement_order_check();
