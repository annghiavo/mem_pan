ALTER TABLE folders
    ADD COLUMN is_public BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX idx_folders_is_public ON folders(is_public) WHERE is_public = TRUE;
