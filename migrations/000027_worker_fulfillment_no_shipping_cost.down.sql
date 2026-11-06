-- Restores the Admin-only payment/terms rule and an all-zero shipping_cost column
-- (dropped values cannot be recovered). Refuses if a Worker already did that work.
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM fulfillment_event e JOIN account a ON a.id=e.actor_id WHERE e.action IN ('terms','cash','review') AND a.role='worker')
  OR EXISTS (SELECT 1 FROM payment p JOIN account a ON a.id=p.reviewed_by WHERE a.role='worker') THEN
    RAISE EXCEPTION 'Refusing to restore Admin-only UC9 rules: Worker payment/terms history exists';
  END IF;
END $$;
ALTER TABLE delivery ADD COLUMN shipping_cost numeric(18,2) NOT NULL DEFAULT 0 CHECK(shipping_cost>=0 AND shipping_cost<>'NaN'::numeric);
ALTER TABLE delivery DROP CONSTRAINT IF EXISTS delivery_method_check;
ALTER TABLE delivery ADD CONSTRAINT delivery_method_check CHECK(method IN ('self','third_party','pickup') AND (method<>'pickup' OR shipping_cost=0));

CREATE OR REPLACE FUNCTION guard_customer_payment() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE job_key uuid; total numeric; received numeric;
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'Payment history is immutable' USING ERRCODE='23514'; END IF;
 SELECT job_id,amount INTO job_key,total FROM invoice WHERE id=NEW.invoice_id FOR UPDATE;
 IF job_key IS NULL THEN RAISE EXCEPTION 'Invoice required' USING ERRCODE='23514'; END IF;
 IF TG_OP='UPDATE' AND (OLD.status<>'pending' OR NEW.status='pending' OR (to_jsonb(NEW)-'status'-'reviewed_by'-'reviewed_at'-'review_event_id'-'reason') IS DISTINCT FROM (to_jsonb(OLD)-'status'-'reviewed_by'-'reviewed_at'-'review_event_id'-'reason')) THEN RAISE EXCEPTION 'Payment submission/review immutable' USING ERRCODE='23514'; END IF;
 IF NEW.reviewed_by IS NOT NULL AND NOT EXISTS(SELECT 1 FROM account WHERE id=NEW.reviewed_by AND is_active AND role='admin') THEN RAISE EXCEPTION 'Admin review required' USING ERRCODE='23514'; END IF;
 IF TG_OP='INSERT' AND NOT EXISTS(SELECT 1 FROM account a JOIN job j ON j.id=job_key JOIN job_request r ON r.id=j.request_id WHERE a.id=NEW.submitted_by AND a.is_active AND ((NEW.method='cash' AND a.role='admin') OR (NEW.method='bank' AND a.role='customer' AND EXISTS(SELECT 1 FROM customer_member WHERE account_id=a.id AND customer_id=r.customer_id)))) THEN RAISE EXCEPTION 'Payment permission required' USING ERRCODE='23514'; END IF;
 IF NEW.evidence_id IS NOT NULL AND NOT EXISTS(SELECT 1 FROM customer_evidence WHERE id=NEW.evidence_id AND job_id=job_key AND uploaded_by=NEW.submitted_by AND state='ready') THEN RAISE EXCEPTION 'Own ready private proof required' USING ERRCODE='23514'; END IF;
 SELECT COALESCE(sum(amount),0) INTO received FROM payment_allocation WHERE invoice_id=NEW.invoice_id;
 IF (TG_OP='INSERT' OR NEW.status='approved') AND NEW.amount>total-received THEN RAISE EXCEPTION 'Payment exceeds remaining balance' USING ERRCODE='23514'; END IF;RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION check_fulfillment_job() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE j job;e fulfillment_event;
BEGIN
 SELECT * INTO j FROM job WHERE id=NEW.id;IF j.fulfillment_version=0 THEN RETURN NULL;END IF;
 SELECT * INTO e FROM fulfillment_event WHERE job_id=j.id AND version=j.fulfillment_version;
 IF e.id IS NULL OR NOT EXISTS(SELECT 1 FROM account WHERE id=e.actor_id AND ((e.action IN ('terms','cash','review') AND role='admin') OR(e.action IN ('invoice','ship') AND role IN ('admin','worker')) OR(e.action IN ('accept','return','bank') AND role='customer'))) THEN RAISE EXCEPTION 'Fulfillment audit/role required' USING ERRCODE='23514';END IF;
 IF (e.action='invoice' AND NOT EXISTS(SELECT 1 FROM invoice WHERE id=e.id AND job_id=j.id)) OR(e.action='ship' AND NOT EXISTS(SELECT 1 FROM delivery WHERE id=e.id AND job_id=j.id)) OR(e.action IN ('accept','return') AND NOT EXISTS(SELECT 1 FROM delivery WHERE decision_event_id=e.id AND job_id=j.id AND decided_by=e.actor_id AND status=(CASE WHEN e.action='accept' THEN 'delivered' ELSE 'returned' END))) THEN RAISE EXCEPTION 'Fulfillment detail audit required' USING ERRCODE='23514'; END IF;RETURN NULL;
END $$;
