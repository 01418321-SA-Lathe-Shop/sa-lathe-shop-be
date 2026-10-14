CREATE TABLE person (
    id uuid PRIMARY KEY,
    name text NOT NULL CHECK (length(btrim(name)) BETWEEN 1 AND 200),
    phone text,
    email text,
    created_at timestamptz NOT NULL DEFAULT now(),
    edited_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX person_phone_unique ON person(phone) WHERE phone IS NOT NULL;
CREATE UNIQUE INDEX person_email_unique ON person(lower(email)) WHERE email IS NOT NULL;

CREATE TABLE staff_account (
    id uuid PRIMARY KEY,
    person_id uuid NOT NULL UNIQUE REFERENCES person(id) ON DELETE RESTRICT,
    username text NOT NULL UNIQUE CHECK (username ~ '^[a-z0-9][a-z0-9_.-]{2,63}$'),
    password_hash text NOT NULL,
    role text NOT NULL CHECK (role IN ('admin', 'worker')),
    is_active boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    edited_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE admin (
    person_id uuid PRIMARY KEY REFERENCES person(id) ON DELETE RESTRICT
);
CREATE TABLE worker (
    person_id uuid PRIMARY KEY REFERENCES person(id) ON DELETE RESTRICT,
    employee_no text NOT NULL UNIQUE CHECK (employee_no ~ '^[A-Z0-9][A-Z0-9_.-]{0,29}$'),
    position text NOT NULL DEFAULT '' CHECK (length(position) <= 100),
    hired_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE auth_session (
    id uuid PRIMARY KEY,
    account_id uuid NOT NULL REFERENCES staff_account(id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz,
    CHECK (expires_at > created_at)
);
CREATE INDEX auth_session_active_account ON auth_session(account_id) WHERE revoked_at IS NULL;
CREATE INDEX auth_session_expires ON auth_session(expires_at);
COMMENT ON TABLE staff_account IS 'Admin/Worker identity only; customer authentication is undecided';
COMMENT ON COLUMN staff_account.password_hash IS 'Salted Argon2id PHC string; never returned by the API';
COMMENT ON COLUMN auth_session.token_hash IS 'SHA-256 digest of a random opaque session token; raw token is never stored';
COMMENT ON COLUMN staff_account.is_active IS 'Single source for staff login/access status';
