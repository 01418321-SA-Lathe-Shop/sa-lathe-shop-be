CREATE TABLE quotation (
 id uuid PRIMARY KEY,
 quotation_no bigint GENERATED ALWAYS AS IDENTITY UNIQUE,
 request_id uuid NOT NULL REFERENCES job_request(id) ON DELETE RESTRICT,
 version bigint NOT NULL CHECK(version>0),
 previous_quotation_id uuid UNIQUE,
 status text NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','sent','approved','rejected')),
 lock_version bigint NOT NULL DEFAULT 1 CHECK(lock_version>0),
 created_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 edited_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 created_at timestamptz NOT NULL DEFAULT now(),
 edited_at timestamptz NOT NULL DEFAULT now(),
 sent_at timestamptz,
 UNIQUE(id,request_id),
 UNIQUE(request_id,version),
 FOREIGN KEY(previous_quotation_id,request_id) REFERENCES quotation(id,request_id) ON DELETE RESTRICT,
 CHECK(previous_quotation_id IS NULL OR previous_quotation_id<>id),
 CHECK((version=1)=(previous_quotation_id IS NULL)),
 CHECK((status='draft')=(sent_at IS NULL))
);
CREATE UNIQUE INDEX quotation_one_active ON quotation(request_id) WHERE status IN ('draft','sent','approved');
CREATE INDEX quotation_request_history ON quotation(request_id,version DESC);
CREATE TABLE quotation_cost (
 quotation_id uuid PRIMARY KEY REFERENCES quotation(id) ON DELETE RESTRICT,
 material_cost numeric(18,2) NOT NULL CHECK(material_cost>=0 AND material_cost<>'NaN'::numeric),
 labor_cost numeric(18,2) NOT NULL CHECK(labor_cost>=0 AND labor_cost<>'NaN'::numeric),
 final_price numeric(18,2) NOT NULL CHECK(final_price>=0 AND final_price<>'NaN'::numeric),
 CHECK(final_price>=ceil((material_cost+labor_cost)*160)/100)
);
COMMENT ON TABLE quotation_cost IS 'Whole-request THB totals, no VAT; minimum is total cost plus 60%, rounded upward to a satang';
COMMENT ON COLUMN quotation.lock_version IS 'Optimistic edit/send token; distinct from commercial quotation revision version';
CREATE TABLE request_rejection (
 request_id uuid PRIMARY KEY REFERENCES job_request(id) ON DELETE RESTRICT,
 rejected_by uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 reason text NOT NULL CHECK(length(btrim(reason)) BETWEEN 1 AND 2000),
 created_at timestamptz NOT NULL DEFAULT now()
);
COMMENT ON TABLE request_rejection IS 'Worker UC2 refusal of a pending request, not UC3 customer quotation rejection';
