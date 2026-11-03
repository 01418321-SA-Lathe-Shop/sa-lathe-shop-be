-- Customer addresses. Sign-up records the first one; owners add more later. Existing billing addresses are carried over.
CREATE TABLE customer_address (
 id uuid PRIMARY KEY,
 customer_id uuid NOT NULL REFERENCES customer(id) ON DELETE RESTRICT,
 address text NOT NULL CHECK(length(btrim(address)) BETWEEN 1 AND 500),
 created_by uuid REFERENCES account(id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX customer_address_customer_idx ON customer_address(customer_id,created_at);
INSERT INTO customer_address(id,customer_id,address) SELECT gen_random_uuid(),id,billing_address FROM customer WHERE btrim(billing_address)<>'';
