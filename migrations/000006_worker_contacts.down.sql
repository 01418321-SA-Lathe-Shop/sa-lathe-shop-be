-- Do not silently discard shared contacts to restore the old uniqueness policy.
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM person WHERE email IS NOT NULL GROUP BY lower(email) HAVING count(*)>1)
 OR EXISTS (SELECT 1 FROM person WHERE phone IS NOT NULL GROUP BY phone HAVING count(*)>1) THEN
  RAISE EXCEPTION 'Cannot rollback: shared contacts exist; use a planned forward migration';
 END IF;
END $$;
DROP TRIGGER worker_contacts_preserved ON person;
DROP FUNCTION prevent_worker_contact_removal();
DROP TRIGGER worker_contacts_required ON worker;
DROP FUNCTION require_worker_contacts();
ALTER TABLE person DROP CONSTRAINT person_email_format;
ALTER TABLE person DROP CONSTRAINT person_phone_format;
CREATE UNIQUE INDEX person_email_unique ON person(lower(email)) WHERE email IS NOT NULL;
CREATE UNIQUE INDEX person_phone_unique ON person(phone) WHERE phone IS NOT NULL;
COMMENT ON COLUMN person.email IS NULL;
COMMENT ON COLUMN person.phone IS NULL;
