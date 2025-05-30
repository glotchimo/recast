CREATE TABLE guilds (
    id text PRIMARY KEY,
    name text NOT NULL DEFAULT ''::text,
    settings jsonb NOT NULL DEFAULT '{}'::jsonb,
    created timestamp without time zone NOT NULL,
    updated timestamp without time zone NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted timestamp without time zone
);

CREATE TABLE interactions (
    interaction jsonb NOT NULL,
    created timestamp without time zone NOT NULL
);
