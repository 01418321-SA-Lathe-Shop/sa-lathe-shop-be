-- Read-only summary for confirming that the demo dataset exposes the intended states.
-- Run after both account and workflow seeds have completed.
SELECT entity, status, records
FROM (
  SELECT 'job_request'::text entity, status::text, count(*)::bigint records FROM job_request GROUP BY status
  UNION ALL SELECT 'quotation', status::text, count(*) FROM quotation GROUP BY status
  UNION ALL SELECT 'job', status::text, count(*) FROM job GROUP BY status
  UNION ALL SELECT 'delivery', status::text, count(*) FROM delivery GROUP BY status
  UNION ALL SELECT 'invoice', status::text, count(*) FROM invoice GROUP BY status
  UNION ALL SELECT 'payment', status::text, count(*) FROM payment GROUP BY status
  UNION ALL SELECT 'material_order', status::text, count(*) FROM material_order GROUP BY status
  UNION ALL SELECT 'material_order_item', status::text, count(*) FROM material_order_item GROUP BY status
  UNION ALL SELECT 'material_defect_log', status::text, count(*) FROM material_defect_log GROUP BY status
) states
ORDER BY entity, status;
