-- Reuse person contacts; do not fabricate contacts or invalidate legacy logins.
DROP INDEX person_email_unique;
DROP INDEX person_phone_unique;

ALTER TABLE person ADD CONSTRAINT person_email_format CHECK (
 email IS NULL OR (octet_length(email) BETWEEN 3 AND 254 AND email = btrim(email)
 AND email ~ '^[^[:space:]@<>]+@[^[:space:]@<>]+$')
) NOT VALID;
ALTER TABLE person ADD CONSTRAINT person_phone_format CHECK (
 phone IS NULL OR phone ~ '^\+?[0-9]{7,15}$'
) NOT VALID;

CREATE FUNCTION require_worker_contacts() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT EXISTS (SELECT 1 FROM person WHERE id=NEW.person_id AND email IS NOT NULL AND phone IS NOT NULL) THEN
  RAISE EXCEPTION 'Worker email and phone are required' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER worker_contacts_required BEFORE INSERT OR UPDATE OF person_id ON worker
 FOR EACH ROW EXECUTE FUNCTION require_worker_contacts();

CREATE FUNCTION prevent_worker_contact_removal() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF ((OLD.email IS NOT NULL AND NEW.email IS NULL) OR (OLD.phone IS NOT NULL AND NEW.phone IS NULL))
 AND EXISTS (SELECT 1 FROM worker WHERE person_id=OLD.id) THEN
  RAISE EXCEPTION 'Worker contacts cannot be removed' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END;
$$;
CREATE TRIGGER worker_contacts_preserved BEFORE UPDATE OF email,phone ON person
 FOR EACH ROW EXECUTE FUNCTION prevent_worker_contact_removal();

COMMENT ON COLUMN person.email IS 'Contact only, not unique or a login identifier; required for new Workers';
COMMENT ON COLUMN person.phone IS 'Contact only, optional leading + and 7-15 digits; not unique; legacy NULL retained';
COMMENT ON FUNCTION require_worker_contacts() IS 'New Worker profiles require contacts; pre-existing NULL contacts are preserved without fake backfills';
