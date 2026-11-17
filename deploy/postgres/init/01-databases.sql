-- F001 T010 — one PostgreSQL cluster, one database per module (ADR-0037, ADR-0074 §4).
-- Runs once, as superuser, on first container start (docker-entrypoint-initdb.d).
-- Ownership moves to <module>_migrator in 02-roles.sh after the roles exist.

CREATE DATABASE eduworld_identity;
CREATE DATABASE eduworld_learning;
CREATE DATABASE eduworld_game;
CREATE DATABASE eduworld_competition;
CREATE DATABASE eduworld_chat;
CREATE DATABASE keycloak;

-- Scratch database for Atlas (`migrate diff` / `migrate lint` --dev-url). Never holds data.
CREATE DATABASE atlas_dev;
