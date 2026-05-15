DROP INDEX IF EXISTS idx_folders_is_public;

ALTER TABLE folders
    DROP COLUMN IF EXISTS is_public;
