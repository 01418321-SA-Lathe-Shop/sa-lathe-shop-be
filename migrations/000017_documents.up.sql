-- Printed-document details. shop_profile stays empty until the first Admin save (the demo seed needs empty tables). Empty text means "not provided"; documents print a fill-in line.
CREATE TABLE shop_profile (
 id boolean PRIMARY KEY DEFAULT true CHECK(id),
 name text NOT NULL DEFAULT '' CHECK(length(name)<=200),
 address text NOT NULL DEFAULT '' CHECK(length(address)<=500),
 tax_id text NOT NULL DEFAULT '' CHECK(length(tax_id)<=30),
 phone text NOT NULL DEFAULT '' CHECK(length(phone)<=64),
 email text NOT NULL DEFAULT '' CHECK(length(email)<=254),
 bank_name text NOT NULL DEFAULT '' CHECK(length(bank_name)<=200),
 bank_account_name text NOT NULL DEFAULT '' CHECK(length(bank_account_name)<=200),
 bank_account_no text NOT NULL DEFAULT '' CHECK(length(bank_account_no)<=64),
 lock_version bigint NOT NULL DEFAULT 1,
 updated_by uuid REFERENCES account(id) ON DELETE RESTRICT,
 updated_at timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE customer
 ADD COLUMN billing_address text NOT NULL DEFAULT '' CHECK(length(billing_address)<=500),
 ADD COLUMN tax_id text NOT NULL DEFAULT '' CHECK(length(tax_id)<=30),
 ADD COLUMN billing_phone text NOT NULL DEFAULT '' CHECK(length(billing_phone)<=64),
 ADD COLUMN billing_email text NOT NULL DEFAULT '' CHECK(length(billing_email)<=254),
 ADD COLUMN billing_lock_version bigint NOT NULL DEFAULT 1;

ALTER TABLE quotation
 ADD COLUMN valid_until date,
 ADD COLUMN notes text NOT NULL DEFAULT '' CHECK(length(notes)<=2000);

CREATE TABLE customer_po (
 quotation_id uuid PRIMARY KEY REFERENCES quotation_acceptance_terms(quotation_id) ON DELETE RESTRICT,
 po_no text NOT NULL CHECK(length(btrim(po_no)) BETWEEN 1 AND 64),
 issued_on date NOT NULL,
 remarks text NOT NULL DEFAULT '' CHECK(length(remarks)<=2000),
 created_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 updated_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 lock_version bigint NOT NULL DEFAULT 1,
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
