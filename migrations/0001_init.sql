-- +goose Up
-- Bootstrap migration. Real schema lands in 0002+ (Phase 1 Steps 4–6).
-- goose requires at least one migration for its tracking table to exist;
-- this is a no-op so future migrations have something to build on.
SELECT 1;

-- +goose Down
SELECT 1;
