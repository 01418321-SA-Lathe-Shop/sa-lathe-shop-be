DROP TABLE customer_po;
ALTER TABLE quotation DROP COLUMN valid_until,DROP COLUMN notes;
ALTER TABLE customer DROP COLUMN billing_address,DROP COLUMN tax_id,DROP COLUMN billing_phone,DROP COLUMN billing_email,DROP COLUMN billing_lock_version;
DROP TABLE shop_profile;
