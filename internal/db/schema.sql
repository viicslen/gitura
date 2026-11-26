-- pr_state stores per-PR app-managed state (not user-editable).
CREATE TABLE IF NOT EXISTS pr_state (
    owner      TEXT    NOT NULL,
    repo       TEXT    NOT NULL,
    number     INTEGER NOT NULL,
    local_path TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (owner, repo, number)
);

-- submodule_file_viewed stores viewed marks on files inside a PR's submodule bumps that no submodule PR
-- can hold; a mark lasts while the file's change matches fingerprint. submodule is the repo listing path,
-- since one path can be in two (a directory turned into a nested submodule); pointer tells a nested
-- submodule's pointer row from a regular file at its path.
CREATE TABLE IF NOT EXISTS submodule_file_viewed (
    owner       TEXT    NOT NULL,
    repo        TEXT    NOT NULL,
    number      INTEGER NOT NULL,
    submodule   TEXT    NOT NULL,
    path        TEXT    NOT NULL,
    pointer     BOOLEAN NOT NULL DEFAULT 0,
    fingerprint TEXT    NOT NULL,
    PRIMARY KEY (owner, repo, number, submodule, path, pointer)
);
