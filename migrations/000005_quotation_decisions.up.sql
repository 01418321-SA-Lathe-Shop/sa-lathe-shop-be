CREATE TABLE quotation_decision (
 quotation_id uuid PRIMARY KEY,
 request_id uuid NOT NULL,
 actor_id uuid NOT NULL REFERENCES account(id) ON DELETE RESTRICT,
 outcome text NOT NULL CHECK(outcome IN ('accept','reject')),
 reason text NOT NULL DEFAULT '' CHECK(length(reason)<=2000),
 expected_lock_version bigint NOT NULL CHECK(expected_lock_version>0),
 decided_at timestamptz NOT NULL,
 rejection_count integer NOT NULL,
 request_status text NOT NULL CHECK(request_status IN ('ongoing','canceled')),
 FOREIGN KEY(quotation_id,request_id) REFERENCES quotation(id,request_id) ON DELETE RESTRICT,
 UNIQUE(quotation_id,outcome),
 UNIQUE(quotation_id,decided_at),
 CHECK(outcome='reject' OR reason=''),
 CHECK((outcome='accept' AND rejection_count BETWEEN 0 AND 5) OR (outcome='reject' AND rejection_count BETWEEN 1 AND 6)),
 CHECK(request_status=CASE WHEN outcome='reject' AND rejection_count=6 THEN 'canceled' ELSE 'ongoing' END)
);
CREATE INDEX quotation_decision_history ON quotation_decision(request_id,decided_at,quotation_id);
CREATE UNIQUE INDEX quotation_rejection_ordinal ON quotation_decision(request_id,rejection_count) WHERE outcome='reject';
CREATE TABLE quotation_acceptance_terms (
 quotation_id uuid PRIMARY KEY,
 outcome text NOT NULL DEFAULT 'accept' CHECK(outcome='accept'),
 customer_type text NOT NULL CHECK(customer_type IN ('individual','company')),
 final_price numeric(18,2) NOT NULL CHECK(final_price>=0 AND final_price<>'NaN'::numeric),
 deposit_required boolean NOT NULL,
 po_required boolean NOT NULL,
 deposit_amount numeric(18,2) NOT NULL CHECK(deposit_amount>=0 AND deposit_amount<>'NaN'::numeric),
 accepted_at timestamptz NOT NULL,
 documents_due_at timestamptz NOT NULL,
 FOREIGN KEY(quotation_id,outcome) REFERENCES quotation_decision(quotation_id,outcome) ON DELETE RESTRICT,
 FOREIGN KEY(quotation_id,accepted_at) REFERENCES quotation_decision(quotation_id,decided_at) ON DELETE RESTRICT,
 CHECK(deposit_required=(customer_type='individual' OR final_price>10000)),
 CHECK(po_required=(customer_type='company')),
 CHECK(deposit_amount=CASE WHEN deposit_required THEN ceil(final_price*50)/100 ELSE 0 END),
 CHECK(deposit_amount<=final_price),
 CHECK(documents_due_at=accepted_at+interval '336 hours')
);
COMMENT ON TABLE quotation_decision IS 'One immutable customer decision per quotation; count is request-wide across revisions, not draft saves or retries';
COMMENT ON TABLE quotation_acceptance_terms IS 'Acceptance snapshot only, not payment receipt, verified PO, or production job';
