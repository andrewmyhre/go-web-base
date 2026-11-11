-- +goose Up

CREATE TABLE example_greetings (
    id         bigserial   PRIMARY KEY,
    greeting   text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down

DROP TABLE example_greetings;
