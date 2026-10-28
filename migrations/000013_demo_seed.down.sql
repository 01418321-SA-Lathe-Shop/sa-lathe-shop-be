DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM demo_seed_dataset) THEN
  RAISE EXCEPTION 'Refusing to discard demo seed provenance; safely clean the unused dataset first';
 END IF;
END $$;
DROP TABLE demo_seed_step;
DROP TABLE demo_seed_record;
DROP TABLE demo_seed_dataset;
