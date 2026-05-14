-- +goose Up

ALTER TABLE scan_runs ADD COLUMN last_completed_folder TEXT;

-- +goose Down

ALTER TABLE scan_runs DROP COLUMN last_completed_folder;
