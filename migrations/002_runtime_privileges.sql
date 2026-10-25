-- Ce rôle est créé par Compose ; sa présence n’est pas supposée ailleurs.
DO $$
BEGIN
 IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='streamops_app') THEN
  REVOKE INSERT, UPDATE ON schema_migrations FROM streamops_app;
  REVOKE UPDATE ON incident_events FROM streamops_app;
 END IF;
END $$;
