ALTER TABLE reports ADD COLUMN IF NOT EXISTS assigned_to UUID;
CREATE INDEX IF NOT EXISTS idx_reports_assigned ON reports(assigned_to)
    WHERE status IN ('pending', 'reviewing');
