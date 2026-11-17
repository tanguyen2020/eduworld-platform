-- F001 T030 — no cross-database access and no implicit privileges (NFR-DEPL-002, ADR-0028, ADR-0074 §4).
-- PUBLIC loses CONNECT on every module database; 02-roles.sh grants CONNECT to the module's own roles only.

REVOKE CONNECT, TEMPORARY ON DATABASE eduworld_identity    FROM PUBLIC;
REVOKE CONNECT, TEMPORARY ON DATABASE eduworld_learning    FROM PUBLIC;
REVOKE CONNECT, TEMPORARY ON DATABASE eduworld_game        FROM PUBLIC;
REVOKE CONNECT, TEMPORARY ON DATABASE eduworld_competition FROM PUBLIC;
REVOKE CONNECT, TEMPORARY ON DATABASE eduworld_chat        FROM PUBLIC;
REVOKE CONNECT, TEMPORARY ON DATABASE keycloak             FROM PUBLIC;

-- The public schema is not used by modules; nobody but the owner may create objects in it.
\connect eduworld_identity
REVOKE ALL ON SCHEMA public FROM PUBLIC;
\connect eduworld_learning
REVOKE ALL ON SCHEMA public FROM PUBLIC;
\connect eduworld_game
REVOKE ALL ON SCHEMA public FROM PUBLIC;
\connect eduworld_competition
REVOKE ALL ON SCHEMA public FROM PUBLIC;
\connect eduworld_chat
REVOKE ALL ON SCHEMA public FROM PUBLIC;
