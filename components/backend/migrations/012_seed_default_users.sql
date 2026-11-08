-- +goose Up
INSERT INTO users (username, password_hash, role, nama_instansi)
VALUES ('admin-cabdin', '$2y$10$dnsYhNMJQxaV1tBHY0QHHuUJbMX/Lr7J13KjZxMRU2wDtli0HHxGC', 'ADMIN', 'Cabdin Bangkalan')
ON CONFLICT (username) DO NOTHING;

INSERT INTO users (username, password_hash, role, school_id, npsn_instansi, nama_instansi)
SELECT 'admin-SMAN1bangkalan', '$2y$10$q.lel1WNpmnphmzqJ4rJ2Ot49LJnChQjo.ZC5JoHqjImI/s.g8vS2', 'SCHOOL_ADMIN', id, npsn_sekolah, nama_sekolah
FROM data_sekolah
WHERE npsn_sekolah = '20531146'
LIMIT 1
ON CONFLICT (username) DO NOTHING;

-- +goose Down
DELETE FROM users WHERE username IN ('admin-cabdin', 'admin-SMAN1bangkalan');
