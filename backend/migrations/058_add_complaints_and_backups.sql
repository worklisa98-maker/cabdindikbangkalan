-- +goose Up
-- +goose StatementBegin
CREATE TABLE complaint_categories (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    report_type VARCHAR(16) NOT NULL CHECK (report_type IN ('PENGADUAN','KRITIK','SARAN')),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (name, report_type)
);

INSERT INTO complaint_categories (name, report_type) VALUES
('Pelayanan', 'PENGADUAN'), ('Etika/Perilaku', 'PENGADUAN'), ('Sarana Prasarana', 'PENGADUAN'),
('Pelayanan', 'KRITIK'), ('Kinerja', 'KRITIK'), ('Aplikasi Digital', 'KRITIK'),
('Pelayanan', 'SARAN'), ('Pendidikan', 'SARAN'), ('Aplikasi Digital', 'SARAN')
ON CONFLICT DO NOTHING;

CREATE TABLE complaints (
    id BIGSERIAL PRIMARY KEY,
    report_id VARCHAR(32) NOT NULL UNIQUE,
    pin_hash TEXT NOT NULL,
    report_type VARCHAR(16) NOT NULL CHECK (report_type IN ('PENGADUAN','KRITIK','SARAN')),
    category_id BIGINT REFERENCES complaint_categories(id),
    title VARCHAR(200) NOT NULL,
    target_type VARCHAR(24) NOT NULL,
    school_id INTEGER REFERENCES data_sekolah(id),
    target_name VARCHAR(255),
    event_date DATE,
    event_location VARCHAR(255),
    related_parties VARCHAR(500),
    details TEXT NOT NULL CHECK (char_length(details) <= 10000),
    impact TEXT CHECK (char_length(impact) <= 5000),
    expected_resolution TEXT CHECK (char_length(expected_resolution) <= 5000),
    reason TEXT CHECK (char_length(reason) <= 5000),
    expected_benefit TEXT CHECK (char_length(expected_benefit) <= 5000),
    confidentiality VARCHAR(16) NOT NULL CHECK (confidentiality IN ('OPEN','CONFIDENTIAL','ANONYMOUS')),
    reporter_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    reporter_name VARCHAR(200),
    reporter_username VARCHAR(100),
    reporter_role VARCHAR(50),
    reporter_institution VARCHAR(255),
    reporter_whatsapp VARCHAR(40),
    reporter_email VARCHAR(255),
    assigned_to INTEGER REFERENCES users(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'RECEIVED' CHECK (status IN
      ('RECEIVED','VERIFICATION','FOLLOW_UP','RESPONDED','COMPLETED','NEEDS_INFO','REJECTED_SPAM','OUT_OF_SCOPE','REOPENED')),
    truth_confirmed BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX complaints_owner_idx ON complaints(reporter_user_id, created_at DESC);
CREATE INDEX complaints_assignment_idx ON complaints(assigned_to, status);
CREATE INDEX complaints_filter_idx ON complaints(report_type, category_id, status, created_at DESC);

CREATE TABLE complaint_attachments (
    id BIGSERIAL PRIMARY KEY,
    complaint_id BIGINT NOT NULL REFERENCES complaints(id) ON DELETE CASCADE,
    timeline_id BIGINT,
    storage_key TEXT NOT NULL UNIQUE,
    original_name VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL CHECK (size_bytes BETWEEN 1 AND 1048576),
    sha256 CHAR(64) NOT NULL,
    public_to_reporter BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE complaint_timeline (
    id BIGSERIAL PRIMARY KEY,
    complaint_id BIGINT NOT NULL REFERENCES complaints(id) ON DELETE CASCADE,
    actor_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    old_status VARCHAR(32),
    new_status VARCHAR(32),
    message TEXT CHECK (char_length(message) <= 5000),
    is_internal BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX complaint_timeline_report_idx ON complaint_timeline(complaint_id, created_at);

CREATE TABLE complaint_popup_images (
    id BIGSERIAL PRIMARY KEY,
    title VARCHAR(200) NOT NULL,
    alt_text VARCHAR(300) NOT NULL,
    storage_key TEXT NOT NULL UNIQUE,
    mime_type VARCHAR(100) NOT NULL,
    size_bytes BIGINT NOT NULL,
    is_active BOOLEAN NOT NULL DEFAULT FALSE,
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX complaint_popup_one_active_idx ON complaint_popup_images(is_active) WHERE is_active;

CREATE TABLE complaint_audit_log (
    id BIGSERIAL PRIMARY KEY,
    complaint_id BIGINT NOT NULL REFERENCES complaints(id) ON DELETE CASCADE,
    actor_user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(80) NOT NULL,
    old_value JSONB,
    new_value JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE backup_history (
    id BIGSERIAL PRIMARY KEY,
    requested_by INTEGER NOT NULL REFERENCES users(id),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status VARCHAR(16) NOT NULL CHECK (status IN ('RUNNING','SUCCESS','FAILED')),
    size_bytes BIGINT,
    checksum_sha256 CHAR(64),
    safe_error VARCHAR(500),
    duration_ms BIGINT
);
CREATE UNIQUE INDEX backup_single_running_idx ON backup_history(status) WHERE status = 'RUNNING';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS backup_history, complaint_audit_log, complaint_popup_images,
 complaint_attachments, complaint_timeline, complaints, complaint_categories CASCADE;
-- +goose StatementEnd
