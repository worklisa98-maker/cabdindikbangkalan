-- +goose Up
WITH legacy_schools AS (
  SELECT DISTINCT school_id
  FROM prestasi_siswa
  WHERE import_batch_id IS NULL
),
admin_user AS (
  SELECT id
  FROM users
  WHERE role = 'ADMIN'
  ORDER BY id
  LIMIT 1
)
INSERT INTO uploads (
  school_id,
  user_id,
  type,
  year,
  storage_key,
  original_filename,
  status,
  created_at,
  updated_at
)
SELECT
  ls.school_id,
  au.id,
  'ACHIEVEMENT',
  EXTRACT(YEAR FROM NOW())::INT,
  '',
  'LEGACY_IMPORT_0',
  'SUCCESS',
  NOW(),
  NOW()
FROM legacy_schools ls
CROSS JOIN admin_user au
WHERE NOT EXISTS (
  SELECT 1
  FROM uploads u
  WHERE u.school_id = ls.school_id
    AND u.type = 'ACHIEVEMENT'
    AND u.original_filename = 'LEGACY_IMPORT_0'
);

UPDATE prestasi_siswa ps
SET import_batch_id = u.id
FROM uploads u
WHERE ps.import_batch_id IS NULL
  AND ps.school_id = u.school_id
  AND u.type = 'ACHIEVEMENT'
  AND u.original_filename = 'LEGACY_IMPORT_0';

WITH legacy_schools AS (
  SELECT DISTINCT school_id
  FROM siswa_perguruan_tinggi
  WHERE import_batch_id IS NULL
),
admin_user AS (
  SELECT id
  FROM users
  WHERE role = 'ADMIN'
  ORDER BY id
  LIMIT 1
)
INSERT INTO uploads (
  school_id,
  user_id,
  type,
  year,
  storage_key,
  original_filename,
  status,
  created_at,
  updated_at
)
SELECT
  ls.school_id,
  au.id,
  'ACCEPTANCE',
  EXTRACT(YEAR FROM NOW())::INT,
  '',
  'LEGACY_IMPORT_0',
  'SUCCESS',
  NOW(),
  NOW()
FROM legacy_schools ls
CROSS JOIN admin_user au
WHERE NOT EXISTS (
  SELECT 1
  FROM uploads u
  WHERE u.school_id = ls.school_id
    AND u.type = 'ACCEPTANCE'
    AND u.original_filename = 'LEGACY_IMPORT_0'
);

UPDATE siswa_perguruan_tinggi spt
SET import_batch_id = u.id
FROM uploads u
WHERE spt.import_batch_id IS NULL
  AND spt.school_id = u.school_id
  AND u.type = 'ACCEPTANCE'
  AND u.original_filename = 'LEGACY_IMPORT_0';

-- +goose Down
UPDATE prestasi_siswa ps
SET import_batch_id = NULL
FROM uploads u
WHERE ps.import_batch_id = u.id
  AND u.type = 'ACHIEVEMENT'
  AND u.original_filename = 'LEGACY_IMPORT_0';

UPDATE siswa_perguruan_tinggi spt
SET import_batch_id = NULL
FROM uploads u
WHERE spt.import_batch_id = u.id
  AND u.type = 'ACCEPTANCE'
  AND u.original_filename = 'LEGACY_IMPORT_0';

DELETE FROM uploads
WHERE original_filename = 'LEGACY_IMPORT_0'
  AND type IN ('ACHIEVEMENT', 'ACCEPTANCE');
