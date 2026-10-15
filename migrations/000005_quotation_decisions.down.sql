DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM quotation_decision) THEN
  RAISE EXCEPTION 'Customer decisions exist; export/reconcile before rollback';
 END IF;
END $$;
DROP TABLE quotation_acceptance_terms;
DROP TABLE quotation_decision;
