-- Refuse destructive rollback when Phase 4 has live business data.
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM customer) OR EXISTS(SELECT 1 FROM account WHERE role='customer') THEN
  RAISE EXCEPTION 'Phase 4 contains customer data; export and explicitly reconcile it before rollback';
 END IF;
END $$;
DROP TABLE notification;
DROP TABLE outbox;
DROP TABLE request_attachment;
DROP TABLE job_request;
DROP TABLE customer_invitation;
DROP TABLE customer_member;
DROP TABLE customer;
ALTER TABLE account DROP CONSTRAINT account_role_check;
ALTER TABLE account ADD CONSTRAINT staff_account_role_check CHECK(role IN ('admin','worker'));
ALTER TABLE account RENAME TO staff_account;
