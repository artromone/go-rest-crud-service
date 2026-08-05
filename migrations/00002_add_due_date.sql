-- +goose Up
-- +goose StatementBegin
ALTER TABLE tasks ADD COLUMN due_date TIMESTAMPTZ;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE tasks DROP COLUMN due_date;
-- +goose StatementEnd
