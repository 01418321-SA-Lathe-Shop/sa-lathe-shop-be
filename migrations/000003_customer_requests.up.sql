-- Generalize the existing identity without changing IDs, hashes or sessions.
ALTER TABLE staff_account RENAME TO account;
ALTER TABLE account DROP CONSTRAINT staff_account_role_check;
ALTER TABLE account ADD CONSTRAINT account_role_check CHECK (role IN ('admin','worker','customer'));
COMMENT ON TABLE account IS 'Unified staff/customer login identity; company membership is separate';

CREATE TABLE customer (
 id uuid PRIMARY KEY,
 customer_type text NOT NULL CHECK (customer_type IN ('individual','company')),
 name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE customer_member (
 customer_id uuid NOT NULL REFERENCES customer(id) ON DELETE RESTRICT,
 account_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 is_owner boolean NOT NULL DEFAULT false,
 created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(customer_id,account_id)
);
CREATE INDEX customer_member_account ON customer_member(account_id,customer_id);
CREATE TABLE customer_invitation (
 id uuid PRIMARY KEY,
 customer_id uuid NOT NULL REFERENCES customer(id) ON DELETE RESTRICT,
 created_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash)=32),
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 accepted_at timestamptz,
 CHECK(expires_at>created_at)
);
CREATE INDEX customer_invitation_expiry ON customer_invitation(expires_at) WHERE accepted_at IS NULL;
CREATE TABLE job_request (
 id uuid PRIMARY KEY,
 no_request bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
 customer_id uuid NOT NULL REFERENCES customer(id) ON DELETE RESTRICT,
 created_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 description text NOT NULL CHECK(length(btrim(description)) BETWEEN 1 AND 10000),
 qty bigint NOT NULL CHECK(qty>0),
 material_spec text NOT NULL DEFAULT '' CHECK(length(material_spec)<=2000),
 deadline timestamptz,
 source text NOT NULL DEFAULT 'net' CHECK(source IN ('net','phone','line')),
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','ongoing','completed','canceled')),
 created_at timestamptz NOT NULL DEFAULT now(),
 edited_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(id,customer_id)
);
CREATE INDEX job_request_customer_created ON job_request(customer_id,created_at DESC,id);
CREATE INDEX job_request_staff_queue ON job_request(status,created_at,id);
CREATE TABLE request_attachment (
 id uuid PRIMARY KEY,
 request_id uuid NOT NULL REFERENCES job_request(id) ON DELETE RESTRICT,
 uploaded_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 file_name text NOT NULL CHECK(length(file_name) BETWEEN 1 AND 200),
 content_type text NOT NULL CHECK(content_type IN ('application/pdf','application/msword','application/vnd.openxmlformats-officedocument.wordprocessingml.document','application/vnd.ms-excel','application/vnd.openxmlformats-officedocument.spreadsheetml.sheet','text/plain','image/jpeg','image/png')),
 byte_size bigint NOT NULL CHECK(byte_size BETWEEN 1 AND 100000000),
 staging_key text NOT NULL UNIQUE,
 object_key text NOT NULL UNIQUE,
 state text NOT NULL DEFAULT 'pending' CHECK(state IN ('pending','checking','ready','failed')),
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL,
 completed_at timestamptz,
 CHECK(expires_at>created_at),
 CHECK((state='ready')=(completed_at IS NOT NULL)),
 CHECK(staging_key<>object_key)
);
CREATE INDEX request_attachment_request ON request_attachment(request_id,created_at,id);
COMMENT ON COLUMN request_attachment.object_key IS 'Private immutable final object key; never a URL';
CREATE TABLE notification (
 id uuid PRIMARY KEY,
 account_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 request_id uuid NOT NULL REFERENCES job_request(id) ON DELETE RESTRICT,
 event_id uuid NOT NULL,
 title text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 read_at timestamptz,
 UNIQUE(event_id,account_id)
);
CREATE INDEX notification_inbox ON notification(account_id,created_at DESC,id);
CREATE TABLE outbox (
 id uuid PRIMARY KEY,
 request_id uuid NOT NULL REFERENCES job_request(id) ON DELETE RESTRICT,
 channel text NOT NULL CHECK(channel IN ('in_system','line')),
 attempts integer NOT NULL DEFAULT 0 CHECK(attempts>=0),
 available_at timestamptz NOT NULL DEFAULT now(),
 lease_token uuid,
 lease_until timestamptz,
 completed_at timestamptz,
 failed_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(request_id,channel),
 CHECK((lease_token IS NULL)=(lease_until IS NULL))
);
CREATE INDEX outbox_pending ON outbox(available_at,created_at) WHERE completed_at IS NULL AND failed_at IS NULL;
ALTER TABLE notification ADD CONSTRAINT notification_event_fk FOREIGN KEY(event_id) REFERENCES outbox(id) ON DELETE RESTRICT;
