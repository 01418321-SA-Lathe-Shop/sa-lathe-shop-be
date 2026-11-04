-- Contact/recipient name per customer address; kept apart from the registered customer name. Legacy rows stay empty.
ALTER TABLE customer_address ADD COLUMN contact_name text NOT NULL DEFAULT '' CHECK(length(contact_name)<=200);
