ALTER TABLE customer_address
 ADD COLUMN contact_name text NOT NULL DEFAULT '' CHECK(length(contact_name)<=200);
