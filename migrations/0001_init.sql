-- +goose Up
-- Bootstrap migration. Creates a tiny placeholder so goose's tracking table
-- has a sibling and so sqlc has at least one type to anchor on. Replaced
-- by real schema in Step 4 (identity & libraries) — drop _bootstrap then.
CREATE TABLE _bootstrap (id INT PRIMARY KEY);

-- +goose Down
DROP TABLE _bootstrap;
