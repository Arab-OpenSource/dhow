-- +goose Up
-- One row describing this Dhow installation.
CREATE TABLE instance (
    id                 smallint    PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    created_at         timestamptz NOT NULL DEFAULT now(),
    setup_completed_at timestamptz
);

INSERT INTO instance (id) VALUES (1);

-- +goose Down
DROP TABLE instance;
