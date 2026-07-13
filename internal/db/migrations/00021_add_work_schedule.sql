-- +goose Up
-- +goose StatementBegin

-- User-level work schedule: effective-dated weekday templates + dated exceptions.
-- Destructively retires flat period.target_hours_per_day and the settings that
-- templated it (period.target_hours, window.start). Wipe/reset local DB is OK.

CREATE TABLE work_schedule (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    timezone       TEXT    NOT NULL,                          -- IANA
    workweek_start TEXT    NOT NULL CHECK (workweek_start IN (
        'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'
    )),
    effective_from TEXT    NOT NULL,                          -- YYYY-MM-DD, inclusive
    effective_to   TEXT,                                      -- YYYY-MM-DD, exclusive; NULL = open-ended
    created_at     TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now')),
    CHECK (effective_to IS NULL OR effective_to > effective_from)
);

CREATE TABLE work_schedule_day (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    work_schedule_id  INTEGER NOT NULL REFERENCES work_schedule (id) ON DELETE CASCADE,
    weekday           TEXT    NOT NULL CHECK (weekday IN (
        'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday', 'sunday'
    )),
    expected_minutes  INTEGER NOT NULL CHECK (expected_minutes >= 0),
    UNIQUE (work_schedule_id, weekday)
);

CREATE TABLE work_schedule_window (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    work_schedule_day_id INTEGER NOT NULL REFERENCES work_schedule_day (id) ON DELETE CASCADE,
    start_minutes        INTEGER NOT NULL CHECK (start_minutes >= 0 AND start_minutes < 1440),
    end_minutes          INTEGER NOT NULL CHECK (end_minutes > 0 AND end_minutes <= 1440),
    CHECK (end_minutes > start_minutes)
);

CREATE TABLE schedule_exception (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    date             TEXT    NOT NULL UNIQUE,                 -- YYYY-MM-DD local
    kind             TEXT    NOT NULL CHECK (kind IN ('holiday', 'leave', 'changed_hours')),
    expected_minutes INTEGER NOT NULL CHECK (expected_minutes >= 0),
    created_at       TEXT    NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ', 'now'))
);

CREATE TABLE schedule_exception_window (
    id                    INTEGER PRIMARY KEY AUTOINCREMENT,
    schedule_exception_id INTEGER NOT NULL REFERENCES schedule_exception (id) ON DELETE CASCADE,
    start_minutes         INTEGER NOT NULL CHECK (start_minutes >= 0 AND start_minutes < 1440),
    end_minutes           INTEGER NOT NULL CHECK (end_minutes > 0 AND end_minutes <= 1440),
    CHECK (end_minutes > start_minutes)
);

-- Half-open [effective_from, effective_to) ranges must not overlap.
CREATE TRIGGER work_schedule_no_overlap_insert
BEFORE INSERT ON work_schedule
FOR EACH ROW
BEGIN
    SELECT RAISE(ABORT, 'work_schedule effective range overlaps existing schedule')
    WHERE EXISTS (
        SELECT 1 FROM work_schedule
        WHERE (effective_to IS NULL OR NEW.effective_from < effective_to)
          AND (NEW.effective_to IS NULL OR effective_from < NEW.effective_to)
    );
END;

CREATE TRIGGER work_schedule_no_overlap_update
BEFORE UPDATE OF effective_from, effective_to ON work_schedule
FOR EACH ROW
BEGIN
    SELECT RAISE(ABORT, 'work_schedule effective range overlaps existing schedule')
    WHERE EXISTS (
        SELECT 1 FROM work_schedule
        WHERE id != NEW.id
          AND (effective_to IS NULL OR NEW.effective_from < effective_to)
          AND (NEW.effective_to IS NULL OR effective_from < NEW.effective_to)
    );
END;

ALTER TABLE period DROP COLUMN target_hours_per_day;

DELETE FROM app_setting WHERE key IN ('period.target_hours', 'window.start');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE period ADD COLUMN target_hours_per_day REAL NOT NULL DEFAULT 8;

DROP TRIGGER IF EXISTS work_schedule_no_overlap_update;
DROP TRIGGER IF EXISTS work_schedule_no_overlap_insert;
DROP TABLE schedule_exception_window;
DROP TABLE schedule_exception;
DROP TABLE work_schedule_window;
DROP TABLE work_schedule_day;
DROP TABLE work_schedule;

-- +goose StatementEnd
