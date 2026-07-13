-- name: CreateWorkSchedule :one
INSERT INTO work_schedule (timezone, workweek_start, effective_from, effective_to)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: ListWorkSchedules :many
SELECT * FROM work_schedule
ORDER BY effective_from;

-- name: GetWorkSchedule :one
SELECT * FROM work_schedule WHERE id = ?;

-- name: CountWorkSchedules :one
SELECT COUNT(*) FROM work_schedule;

-- name: CreateWorkScheduleDay :one
INSERT INTO work_schedule_day (work_schedule_id, weekday, expected_minutes)
VALUES (?, ?, ?)
RETURNING *;

-- name: ListWorkScheduleDays :many
SELECT * FROM work_schedule_day
WHERE work_schedule_id = ?
ORDER BY CASE weekday
    WHEN 'monday' THEN 1 WHEN 'tuesday' THEN 2 WHEN 'wednesday' THEN 3
    WHEN 'thursday' THEN 4 WHEN 'friday' THEN 5 WHEN 'saturday' THEN 6
    WHEN 'sunday' THEN 7 END;

-- name: CreateWorkScheduleWindow :one
INSERT INTO work_schedule_window (work_schedule_day_id, start_minutes, end_minutes)
VALUES (?, ?, ?)
RETURNING *;

-- name: ListWorkScheduleWindows :many
SELECT * FROM work_schedule_window
WHERE work_schedule_day_id = ?
ORDER BY start_minutes;

-- name: CreateScheduleException :one
INSERT INTO schedule_exception (date, kind, expected_minutes)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetScheduleExceptionByDate :one
SELECT * FROM schedule_exception WHERE date = ?;

-- name: ListScheduleExceptions :many
SELECT * FROM schedule_exception ORDER BY date;

-- name: CreateScheduleExceptionWindow :one
INSERT INTO schedule_exception_window (schedule_exception_id, start_minutes, end_minutes)
VALUES (?, ?, ?)
RETURNING *;

-- name: ListScheduleExceptionWindows :many
SELECT * FROM schedule_exception_window
WHERE schedule_exception_id = ?
ORDER BY start_minutes;
