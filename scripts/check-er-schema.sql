-- Reports ER table/field names from er_diagram_table_structure.md that are absent.
-- Zero rows means the migrated database exposes every required name.
WITH expected(table_name, column_name) AS (
  VALUES
    ('person','id'),('person','name'),('person','phone'),('person','email'),('person','created_at'),('person','edited_at'),
    ('customer','person_id'),('customer','customer_type'),('customer','tax_id'),
    ('worker','person_id'),('worker','employee_no'),('worker','position'),('worker','status'),('worker','hired_at'),
    ('admin','person_id'),('admin','admin_level'),('admin','status'),
    ('address','id'),('address','customer_id'),('address','address_line1'),('address','address_line2'),('address','district'),('address','province'),('address','postal_code'),('address','is_default'),('address','created_at'),('address','edited_at'),
    ('job_request','id'),('job_request','customer_id'),('job_request','no_request'),('job_request','source'),('job_request','description'),('job_request','qty'),('job_request','material_spec'),('job_request','deadline'),('job_request','status'),('job_request','created_at'),('job_request','edited_at'),
    ('request_attachment','id'),('request_attachment','request_id'),('request_attachment','file_url'),('request_attachment','uploaded_at'),
    ('quotation','id'),('quotation','quotation_no'),('quotation','request_id'),('quotation','version'),('quotation','previous_quotation_id'),('quotation','status'),('quotation','sent_at'),('quotation','decided_at'),
    ('quotation_cost','id'),('quotation_cost','quotation_id'),('quotation_cost','material_cost'),('quotation_cost','labor_cost'),('quotation_cost','final_price'),
    ('purchase_order','id'),('purchase_order','quotation_id'),('purchase_order','file_url'),('purchase_order','uploaded_at'),
    ('deposit','id'),('deposit','quotation_id'),('deposit','percent'),('deposit','amount_applied'),('deposit','paid_at'),('deposit','proof_file_url'),
    ('job','id'),('job','quotation_id'),('job','status'),('job','estimated_cost'),('job','estimated_time'),('job','due_date'),('job','actual_cost'),('job','actual_time'),('job','created_at'),('job','edited_at'),
    ('job_photo','id'),('job_photo','job_id'),('job_photo','url_photo'),('job_photo','uploaded_at'),
    ('job_material_usage','id'),('job_material_usage','job_id'),('job_material_usage','material_id'),('job_material_usage','qty_used'),
    ('material_reservation','id'),('material_reservation','material_id'),('material_reservation','job_id'),('material_reservation','qty_reserved'),('material_reservation','status'),
    ('material','id'),('material','name'),('material','unit_id'),('material','qty_on_hand'),('material','avg_cost'),('material','created_at'),('material','edited_at'),
    ('unit','id'),('unit','name'),
    ('unit_conversion','id'),('unit_conversion','material_id'),('unit_conversion','from_unit_id'),('unit_conversion','to_unit_id'),('unit_conversion','factor'),
    ('delivery','id'),('delivery','job_id'),('delivery','method'),('delivery','delivered_at'),('delivery','receiver_name'),('delivery','shipping_cost'),('delivery','proof_url'),('delivery','status'),('delivery','created_at'),('delivery','edited_at'),
    ('supplier','id'),('supplier','name'),('supplier','phone'),('supplier','email'),('supplier','created_at'),('supplier','edited_at'),
    ('material_order','id'),('material_order','supplier_id'),('material_order','order_no'),('material_order','ordered_at'),('material_order','expected_at'),('material_order','status'),('material_order','total_amount'),('material_order','payment_status'),('material_order','payment_method'),('material_order','payment_proof_url'),('material_order','paid_at'),('material_order','created_at'),('material_order','edited_at'),
    ('material_order_item','id'),('material_order_item','material_order_id'),('material_order_item','material_id'),('material_order_item','qty_ordered'),('material_order_item','qty_received'),('material_order_item','qty_damaged'),('material_order_item','unit_cost'),('material_order_item','status'),('material_order_item','expected_at'),('material_order_item','received_at'),('material_order_item','created_at'),('material_order_item','edited_at'),
    ('stock_movement','id'),('stock_movement','material_id'),('stock_movement','movement_type'),('stock_movement','qty'),('stock_movement','unit_cost'),('stock_movement','balance_qty'),('stock_movement','ref_type'),('stock_movement','ref_id'),('stock_movement','created_by'),('stock_movement','created_at'),
    ('material_defect_log','id'),('material_defect_log','material_order_id'),('material_defect_log','material_order_item_id'),('material_defect_log','material_id'),('material_defect_log','qty_damaged'),('material_defect_log','unit_cost'),('material_defect_log','status'),('material_defect_log','reported_by'),('material_defect_log','created_at'),
    ('invoice','id'),('invoice','job_id'),('invoice','due_date'),('invoice','total_amount'),('invoice','status'),
    ('payment','id'),('payment','amount'),('payment','method'),('payment','proof_url'),('payment','paid_at'),
    ('payment_allocation','id'),('payment_allocation','payment_id'),('payment_allocation','invoice_id'),('payment_allocation','amount')
)
SELECT e.table_name, e.column_name
FROM expected e
LEFT JOIN information_schema.columns c
  ON c.table_schema='public' AND c.table_name=e.table_name AND c.column_name=e.column_name
WHERE c.column_name IS NULL
ORDER BY e.table_name,e.column_name;
