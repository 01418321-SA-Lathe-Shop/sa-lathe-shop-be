-- ER alignment, phase 3: give every ER table the remaining ER column names. Existing columns keep their names, so no
-- query, guard or contract changes; each ER column is a plain column that a trigger keeps equal to its source.
-- The sync triggers are named zz_er_sync so they run after every guard trigger on the table (guards compare the row
-- the caller wrote, not the values the sync adds).

-- Backfill happens with user triggers off, because history guards would otherwise reject the update.
ALTER TABLE job DISABLE TRIGGER USER;
ALTER TABLE job_material_usage DISABLE TRIGGER USER;
ALTER TABLE invoice DISABLE TRIGGER USER;
ALTER TABLE material_order DISABLE TRIGGER USER;
ALTER TABLE material_order_item DISABLE TRIGGER USER;
ALTER TABLE unit_conversion DISABLE TRIGGER USER;
ALTER TABLE stock_movement DISABLE TRIGGER USER;
ALTER TABLE delivery DISABLE TRIGGER USER;
ALTER TABLE job_photo DISABLE TRIGGER USER;
ALTER TABLE request_attachment DISABLE TRIGGER USER;
ALTER TABLE deposit DISABLE TRIGGER USER;
ALTER TABLE purchase_order DISABLE TRIGGER USER;
ALTER TABLE material_defect_log DISABLE TRIGGER USER;
ALTER TABLE payment DISABLE TRIGGER USER;

-- JOB ------------------------------------------------------------------------------------------------
ALTER TABLE job ADD COLUMN due_date timestamptz;
CREATE FUNCTION er_job() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.due_date := NEW.due_at; RETURN NEW; END $$;

-- JOB_MATERIAL_USAGE ---------------------------------------------------------------------------------
ALTER TABLE job_material_usage ADD COLUMN qty_used numeric;
CREATE FUNCTION er_job_material_usage() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.qty_used := NEW.quantity; RETURN NEW; END $$;

-- INVOICE --------------------------------------------------------------------------------------------
ALTER TABLE invoice ADD COLUMN due_date timestamptz, ADD COLUMN total_amount numeric;
CREATE FUNCTION er_invoice() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.due_date := NEW.due_at; NEW.total_amount := NEW.amount; RETURN NEW; END $$;

-- MATERIAL_ORDER -------------------------------------------------------------------------------------
ALTER TABLE material_order ADD COLUMN ordered_at timestamptz, ADD COLUMN edited_at timestamptz;
CREATE FUNCTION er_material_order() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.ordered_at := NEW.created_at; IF TG_OP='UPDATE' THEN NEW.edited_at := now(); END IF; RETURN NEW; END $$;

-- MATERIAL_ORDER_ITEM --------------------------------------------------------------------------------
ALTER TABLE material_order_item ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid(), ADD COLUMN material_order_id uuid;
ALTER TABLE material_order_item ADD CONSTRAINT material_order_item_id_key UNIQUE (id);
CREATE FUNCTION er_material_order_item() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN NEW.material_order_id := NEW.order_id; RETURN NEW; END $$;

-- UNIT_CONVERSION ------------------------------------------------------------------------------------
ALTER TABLE unit_conversion ADD COLUMN from_unit_id uuid REFERENCES unit(id) ON DELETE RESTRICT, ADD COLUMN to_unit_id uuid REFERENCES unit(id) ON DELETE RESTRICT;
CREATE FUNCTION er_unit_conversion() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.from_unit_id := NEW.unit_id; SELECT unit_id INTO NEW.to_unit_id FROM material WHERE id=NEW.material_id; RETURN NEW; END $$;

-- STOCK_MOVEMENT -------------------------------------------------------------------------------------
-- movement_type: the three ER types that exist (PURCHASE_RECEIVE, JOB_ISSUE, ADJUSTMENT); there is no RETURN movement.
ALTER TABLE stock_movement ADD COLUMN movement_type text, ADD COLUMN qty numeric, ADD COLUMN balance_qty numeric,
 ADD COLUMN ref_type text, ADD COLUMN ref_id uuid, ADD COLUMN created_by uuid;
CREATE FUNCTION er_stock_movement() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 NEW.movement_type := CASE NEW.mode WHEN 'purchase_receive' THEN 'PURCHASE_RECEIVE' WHEN 'job_issue' THEN 'JOB_ISSUE' ELSE 'ADJUSTMENT' END;
 NEW.qty := NEW.quantity; NEW.balance_qty := NEW.after_quantity; NEW.created_by := NEW.actor_id;
 NEW.ref_type := CASE WHEN NEW.receipt_line_id IS NOT NULL THEN 'MATERIAL_ORDER' WHEN NEW.job_usage_id IS NOT NULL THEN 'JOB' END;
 NEW.ref_id := COALESCE((SELECT l.order_id FROM material_receipt_line l WHERE l.id=NEW.receipt_line_id),(SELECT u.job_id FROM job_material_usage u WHERE u.id=NEW.job_usage_id));
 RETURN NEW;
END $$;

-- DELIVERY -------------------------------------------------------------------------------------------
-- proof_url has no source today (no delivery proof upload exists); the column is there and stays NULL.
ALTER TABLE delivery ADD COLUMN delivered_at timestamptz, ADD COLUMN proof_url text, ADD COLUMN created_at timestamptz, ADD COLUMN edited_at timestamptz;
CREATE FUNCTION er_delivery() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.created_at := NEW.shipped_at; NEW.edited_at := NEW.decided_at; NEW.delivered_at := CASE WHEN NEW.status='delivered' THEN NEW.decided_at END; RETURN NEW; END $$;

-- JOB_PHOTO / REQUEST_ATTACHMENT (object keys are private; they are never URLs) --------------------
ALTER TABLE job_photo ADD COLUMN url_photo text, ADD COLUMN uploaded_at timestamptz;
CREATE FUNCTION er_job_photo() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.url_photo := NEW.object_key; NEW.uploaded_at := COALESCE(NEW.completed_at,NEW.created_at); RETURN NEW; END $$;
ALTER TABLE request_attachment ADD COLUMN file_url text, ADD COLUMN uploaded_at timestamptz;
CREATE FUNCTION er_request_attachment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.file_url := NEW.object_key; NEW.uploaded_at := COALESCE(NEW.completed_at,NEW.created_at); RETURN NEW; END $$;

-- DEPOSIT / PURCHASE_ORDER ---------------------------------------------------------------------------
ALTER TABLE deposit ADD COLUMN amount_applied numeric(18,2), ADD COLUMN proof_file_url text;
CREATE FUNCTION er_deposit() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.amount_applied := NEW.amount; SELECT object_key INTO NEW.proof_file_url FROM request_attachment WHERE id=NEW.attachment_id; RETURN NEW; END $$;
ALTER TABLE purchase_order ADD COLUMN file_url text, ADD COLUMN uploaded_at timestamptz;
CREATE FUNCTION er_purchase_order() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN NEW.uploaded_at := NEW.submitted_at; SELECT object_key INTO NEW.file_url FROM request_attachment WHERE id=NEW.attachment_id; RETURN NEW; END $$;

-- MATERIAL_DEFECT_LOG --------------------------------------------------------------------------------
-- Filled from the damaged receipt line when the claim row is created: order, item, material, unit cost, who received it and when.
ALTER TABLE material_defect_log ADD COLUMN material_order_id uuid, ADD COLUMN material_order_item_id uuid, ADD COLUMN material_id uuid,
 ADD COLUMN qty_damaged numeric, ADD COLUMN unit_cost numeric, ADD COLUMN reported_by uuid, ADD COLUMN created_at timestamptz;
CREATE FUNCTION er_material_defect_log() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 NEW.qty_damaged := NEW.quantity;
 IF TG_OP='INSERT' THEN
  SELECT l.order_id, l.material_id, i.id, i.unit_cost, r.actor_id, r.created_at
  INTO NEW.material_order_id, NEW.material_id, NEW.material_order_item_id, NEW.unit_cost, NEW.reported_by, NEW.created_at
  FROM material_receipt_line l JOIN material_receipt r ON r.id=l.receipt_id
  LEFT JOIN material_order_item i ON i.order_id=l.order_id AND i.material_id=l.material_id WHERE l.id=NEW.source_line_id;
 END IF;
 RETURN NEW;
END $$;

-- PAYMENT / PAYMENT_ALLOCATION -----------------------------------------------------------------------
ALTER TABLE payment ADD COLUMN proof_url text;
CREATE FUNCTION er_payment() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN SELECT object_key INTO NEW.proof_url FROM customer_evidence WHERE id=NEW.evidence_id; RETURN NEW; END $$;
ALTER TABLE payment_allocation ADD COLUMN id uuid NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE payment_allocation ADD CONSTRAINT payment_allocation_id_key UNIQUE (id);

-- QUOTATION_COST.id (the table is keyed by quotation_id) ---------------------------------------------
ALTER TABLE quotation_cost ADD COLUMN id uuid GENERATED ALWAYS AS (quotation_id) STORED;

-- QUOTATION.decided_at -------------------------------------------------------------------------------
ALTER TABLE quotation ADD COLUMN decided_at timestamptz;
CREATE FUNCTION er_quotation_decided() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN UPDATE quotation SET decided_at=NEW.decided_at WHERE id=NEW.quotation_id; RETURN NULL; END $$;

-- WORKER.status / ADMIN.status (mirror account.is_active) ---------------------------------------------
ALTER TABLE worker ADD COLUMN status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive'));
ALTER TABLE admin ADD COLUMN status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','inactive'));
CREATE FUNCTION er_staff_status() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 SELECT CASE WHEN is_active THEN 'active' ELSE 'inactive' END INTO NEW.status FROM account WHERE person_id=NEW.person_id;
 NEW.status := COALESCE(NEW.status,'active');
 RETURN NEW;
END $$;
CREATE FUNCTION er_account_active() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE s text := CASE WHEN NEW.is_active THEN 'active' ELSE 'inactive' END;
BEGIN
 UPDATE worker SET status=s WHERE person_id=NEW.person_id AND status<>s;
 UPDATE admin SET status=s WHERE person_id=NEW.person_id AND status<>s;
 RETURN NULL;
END $$;

-- CUSTOMER.person_id: the owner's person (a company can have several member accounts) ----------------
ALTER TABLE customer ADD COLUMN person_id uuid REFERENCES person(id) ON DELETE RESTRICT;
CREATE FUNCTION er_customer_owner() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE customer SET person_id=(SELECT person_id FROM account WHERE id=NEW.account_id) WHERE id=NEW.customer_id AND (NEW.is_owner OR person_id IS NULL);
 RETURN NULL;
END $$;

-- Backfill.
UPDATE job SET due_date=due_at;
UPDATE job_material_usage SET qty_used=quantity;
UPDATE invoice SET due_date=due_at, total_amount=amount;
UPDATE material_order SET ordered_at=created_at;
UPDATE material_order_item SET material_order_id=order_id;
UPDATE unit_conversion c SET from_unit_id=c.unit_id, to_unit_id=(SELECT m.unit_id FROM material m WHERE m.id=c.material_id);
UPDATE stock_movement s SET
 movement_type=CASE s.mode WHEN 'purchase_receive' THEN 'PURCHASE_RECEIVE' WHEN 'job_issue' THEN 'JOB_ISSUE' ELSE 'ADJUSTMENT' END,
 qty=s.quantity, balance_qty=s.after_quantity, created_by=s.actor_id,
 ref_type=CASE WHEN s.receipt_line_id IS NOT NULL THEN 'MATERIAL_ORDER' WHEN s.job_usage_id IS NOT NULL THEN 'JOB' END,
 ref_id=COALESCE((SELECT l.order_id FROM material_receipt_line l WHERE l.id=s.receipt_line_id),(SELECT u.job_id FROM job_material_usage u WHERE u.id=s.job_usage_id));
UPDATE delivery SET created_at=shipped_at, edited_at=decided_at, delivered_at=CASE WHEN status='delivered' THEN decided_at END;
UPDATE job_photo SET url_photo=object_key, uploaded_at=COALESCE(completed_at,created_at);
UPDATE request_attachment SET file_url=object_key, uploaded_at=COALESCE(completed_at,created_at);
UPDATE deposit d SET amount_applied=d.amount, proof_file_url=(SELECT a.object_key FROM request_attachment a WHERE a.id=d.attachment_id);
UPDATE purchase_order p SET uploaded_at=p.submitted_at, file_url=(SELECT a.object_key FROM request_attachment a WHERE a.id=p.attachment_id);
UPDATE material_defect_log c SET qty_damaged=c.quantity, material_order_id=l.order_id, material_id=l.material_id,
 material_order_item_id=(SELECT i.id FROM material_order_item i WHERE i.order_id=l.order_id AND i.material_id=l.material_id),
 unit_cost=(SELECT i.unit_cost FROM material_order_item i WHERE i.order_id=l.order_id AND i.material_id=l.material_id),
 reported_by=r.actor_id, created_at=r.created_at
FROM material_receipt_line l JOIN material_receipt r ON r.id=l.receipt_id WHERE l.id=c.source_line_id;
UPDATE payment p SET proof_url=(SELECT e.object_key FROM customer_evidence e WHERE e.id=p.evidence_id);
UPDATE quotation q SET decided_at=(SELECT max(d.decided_at) FROM quotation_decision d WHERE d.quotation_id=q.id);
UPDATE worker w SET status=CASE WHEN a.is_active THEN 'active' ELSE 'inactive' END FROM account a WHERE a.person_id=w.person_id;
UPDATE admin w SET status=CASE WHEN a.is_active THEN 'active' ELSE 'inactive' END FROM account a WHERE a.person_id=w.person_id;
UPDATE customer c SET person_id=(SELECT a.person_id FROM customer_member m JOIN account a ON a.id=m.account_id WHERE m.customer_id=c.id ORDER BY m.is_owner DESC, m.created_at LIMIT 1);

-- Run deferred checks so triggers can be created on the touched tables.
SET CONSTRAINTS ALL IMMEDIATE;

ALTER TABLE job ENABLE TRIGGER USER;
ALTER TABLE job_material_usage ENABLE TRIGGER USER;
ALTER TABLE invoice ENABLE TRIGGER USER;
ALTER TABLE material_order ENABLE TRIGGER USER;
ALTER TABLE material_order_item ENABLE TRIGGER USER;
ALTER TABLE unit_conversion ENABLE TRIGGER USER;
ALTER TABLE stock_movement ENABLE TRIGGER USER;
ALTER TABLE delivery ENABLE TRIGGER USER;
ALTER TABLE job_photo ENABLE TRIGGER USER;
ALTER TABLE request_attachment ENABLE TRIGGER USER;
ALTER TABLE deposit ENABLE TRIGGER USER;
ALTER TABLE purchase_order ENABLE TRIGGER USER;
ALTER TABLE material_defect_log ENABLE TRIGGER USER;
ALTER TABLE payment ENABLE TRIGGER USER;
CREATE TRIGGER zz_er_sync BEFORE INSERT OR UPDATE ON job FOR EACH ROW EXECUTE FUNCTION er_job();
CREATE TRIGGER zz_er_sync BEFORE INSERT ON job_material_usage FOR EACH ROW EXECUTE FUNCTION er_job_material_usage();
CREATE TRIGGER zz_er_sync BEFORE INSERT OR UPDATE ON invoice FOR EACH ROW EXECUTE FUNCTION er_invoice();
CREATE TRIGGER zz_er_sync BEFORE INSERT OR UPDATE ON material_order FOR EACH ROW EXECUTE FUNCTION er_material_order();
CREATE TRIGGER zz_er_sync BEFORE INSERT ON material_order_item FOR EACH ROW EXECUTE FUNCTION er_material_order_item();
CREATE TRIGGER zz_er_sync BEFORE INSERT ON unit_conversion FOR EACH ROW EXECUTE FUNCTION er_unit_conversion();
CREATE TRIGGER zz_er_sync BEFORE INSERT ON stock_movement FOR EACH ROW EXECUTE FUNCTION er_stock_movement();
CREATE TRIGGER zz_er_sync BEFORE INSERT OR UPDATE ON delivery FOR EACH ROW EXECUTE FUNCTION er_delivery();
CREATE TRIGGER zz_er_sync BEFORE INSERT OR UPDATE ON job_photo FOR EACH ROW EXECUTE FUNCTION er_job_photo();
CREATE TRIGGER zz_er_sync BEFORE INSERT OR UPDATE ON request_attachment FOR EACH ROW EXECUTE FUNCTION er_request_attachment();
CREATE TRIGGER zz_er_sync BEFORE INSERT ON deposit FOR EACH ROW EXECUTE FUNCTION er_deposit();
CREATE TRIGGER zz_er_sync BEFORE INSERT ON purchase_order FOR EACH ROW EXECUTE FUNCTION er_purchase_order();
CREATE TRIGGER zz_er_sync BEFORE INSERT OR UPDATE ON material_defect_log FOR EACH ROW EXECUTE FUNCTION er_material_defect_log();
CREATE TRIGGER zz_er_sync BEFORE INSERT ON payment FOR EACH ROW EXECUTE FUNCTION er_payment();
CREATE TRIGGER zz_er_sync AFTER INSERT ON quotation_decision FOR EACH ROW EXECUTE FUNCTION er_quotation_decided();
CREATE TRIGGER zz_er_sync BEFORE INSERT ON worker FOR EACH ROW EXECUTE FUNCTION er_staff_status();
CREATE TRIGGER zz_er_sync BEFORE INSERT ON admin FOR EACH ROW EXECUTE FUNCTION er_staff_status();
CREATE TRIGGER zz_er_sync AFTER UPDATE OF is_active ON account FOR EACH ROW EXECUTE FUNCTION er_account_active();
CREATE TRIGGER zz_er_sync AFTER INSERT ON customer_member FOR EACH ROW EXECUTE FUNCTION er_customer_owner();
