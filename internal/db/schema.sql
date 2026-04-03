-- pr_state stores per-PR app-managed state (not user-editable).
CREATE TABLE IF NOT EXISTS pr_state (
    owner      TEXT    NOT NULL,
    repo       TEXT    NOT NULL,
    number     INTEGER NOT NULL,
    local_path TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (owner, repo, number)
);
