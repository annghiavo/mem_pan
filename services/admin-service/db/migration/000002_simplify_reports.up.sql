-- Drop the unused assigned_to column from reports table.
-- It was never populated or consumed anywhere.
ALTER TABLE reports DROP COLUMN IF EXISTS assigned_to;

-- Drop the partial index that referenced it.
DROP INDEX IF EXISTS idx_reports_assigned;
