-- Operational provenance only; no demo rows or business changes on migration.
CREATE TABLE demo_seed_dataset (
 id uuid PRIMARY KEY,
 name text NOT NULL UNIQUE CHECK(name='frontend-accounts'),
 fixture_version integer NOT NULL CHECK(fixture_version=1),
 environment text NOT NULL CHECK(environment IN ('local','test')),
 status text NOT NULL DEFAULT 'active' CHECK(status IN ('active','retired')),
 created_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE demo_seed_record (
 dataset_id uuid NOT NULL REFERENCES demo_seed_dataset(id) ON DELETE RESTRICT,
 record_key text NOT NULL,
 entity_type text NOT NULL CHECK(entity_type IN ('account','customer','member','invitation')),
 record_id uuid NOT NULL,
 parent_id uuid,
 identity jsonb NOT NULL CHECK(jsonb_typeof(identity)='object'),
 PRIMARY KEY(dataset_id,record_key),
 CHECK((entity_type='member')=(parent_id IS NOT NULL))
);
CREATE UNIQUE INDEX demo_seed_record_identity ON demo_seed_record(entity_type,record_id,COALESCE(parent_id,'00000000-0000-0000-0000-000000000000'::uuid));
CREATE TABLE demo_seed_step (
 dataset_id uuid NOT NULL REFERENCES demo_seed_dataset(id) ON DELETE RESTRICT,
 step_key text NOT NULL CHECK(step_key='accounts-v1'),
 manifest_digest text NOT NULL CHECK(manifest_digest ~ '^[a-f0-9]{64}$'),
 completed_at timestamptz NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(dataset_id,step_key)
);
COMMENT ON TABLE demo_seed_record IS 'Exact seed-owned identity bindings, not passwords/hashes/tokens; membership uses account record_id and customer parent_id. Account identity includes person and role profile IDs.';
COMMENT ON TABLE demo_seed_dataset IS 'Explicit local/test CLI provenance only; normal startup never seeds.';
