-- +goose Up
UPDATE users u
SET role = 'USER_SEKOLAH_GASPOL',
    token_version = token_version + 1,
    updated_at = NOW()
WHERE u.role = 'SCHOOL_ADMIN'
  AND u.school_id IS NOT NULL
  AND LOWER(u.username) LIKE 'gaspol-%'
  AND EXISTS (
    SELECT 1
    FROM gaspol_user_roles gur
    WHERE gur.user_id = u.id
      AND gur.role = 'SCHOOL_ENTRY'
  );

-- +goose Down
UPDATE users u
SET role = 'SCHOOL_ADMIN',
    token_version = token_version + 1,
    updated_at = NOW()
WHERE u.role = 'USER_SEKOLAH_GASPOL'
  AND u.school_id IS NOT NULL
  AND LOWER(u.username) LIKE 'gaspol-%'
  AND EXISTS (
    SELECT 1
    FROM gaspol_user_roles gur
    WHERE gur.user_id = u.id
      AND gur.role = 'SCHOOL_ENTRY'
  );
