-- UC12: Admin levels. The earliest Admin (the bootstrap account) becomes the Super Admin; all others stay regular Admins.
ALTER TABLE admin ADD COLUMN admin_level text NOT NULL DEFAULT 'admin' CHECK (admin_level IN ('admin','super_admin'));
UPDATE admin SET admin_level='super_admin'
 WHERE person_id=(SELECT a.person_id FROM account a WHERE a.role='admin' ORDER BY a.created_at,a.id LIMIT 1);
