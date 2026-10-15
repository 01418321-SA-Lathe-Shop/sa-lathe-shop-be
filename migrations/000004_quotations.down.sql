DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM quotation) OR EXISTS(SELECT 1 FROM request_rejection) THEN
  RAISE EXCEPTION 'Quotation data exists; export/reconcile it before rollback';
 END IF;
END $$;
DROP TABLE request_rejection;
DROP TABLE quotation_cost;
DROP TABLE quotation;
