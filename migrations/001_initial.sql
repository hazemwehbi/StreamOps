CREATE TABLE services (
 id uuid PRIMARY KEY,
 service_key varchar(64) NOT NULL UNIQUE CHECK (service_key ~ '^[a-z0-9][a-z0-9_-]{2,63}$'),
 name varchar(200) NOT NULL CHECK (length(btrim(name)) > 0),
 environment text NOT NULL CHECK (environment IN ('development','staging','production')),
 status text NOT NULL CHECK (status IN ('operational','degraded','outage','maintenance')),
 created_at timestamptz NOT NULL DEFAULT now(),
 updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX services_filter_idx ON services(environment,status,created_at DESC,id);
CREATE TABLE incidents (
 id uuid PRIMARY KEY,
 service_id uuid NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
 title varchar(200) NOT NULL CHECK (length(btrim(title)) > 0),
 description text NOT NULL DEFAULT '' CHECK (octet_length(description) <= 10000),
 severity text NOT NULL CHECK (severity IN ('low','medium','high','critical')),
 status text NOT NULL DEFAULT 'open' CHECK (status IN ('open','acknowledged','resolved')),
 opened_at timestamptz NOT NULL DEFAULT now(),
 acknowledged_at timestamptz,
 resolved_at timestamptz,
 resolution_summary text NOT NULL DEFAULT '' CHECK (octet_length(resolution_summary) <= 4000),
 CHECK ((status='resolved') = (resolved_at IS NOT NULL)),
 CHECK (status <> 'acknowledged' OR acknowledged_at IS NOT NULL),
 CHECK (status <> 'open' OR acknowledged_at IS NULL),
 CHECK (status <> 'resolved' OR length(btrim(resolution_summary)) > 0),
 CHECK (status = 'resolved' OR resolution_summary = ''),
 CHECK (acknowledged_at IS NULL OR acknowledged_at >= opened_at),
 CHECK (resolved_at IS NULL OR resolved_at >= coalesce(acknowledged_at,opened_at))
);
CREATE INDEX incidents_service_idx ON incidents(service_id,opened_at DESC,id);
CREATE INDEX incidents_filter_idx ON incidents(status,severity,opened_at DESC,id);
CREATE INDEX incidents_opened_idx ON incidents(opened_at DESC,id);
CREATE TABLE incident_events (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 incident_id uuid NOT NULL REFERENCES incidents(id),
 from_status text NOT NULL,
 to_status text NOT NULL,
 summary text NOT NULL DEFAULT '',
 occurred_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX incident_events_history_idx ON incident_events(incident_id,id);
CREATE TABLE health_checks (
 id uuid PRIMARY KEY,
 service_id uuid NOT NULL REFERENCES services(id),
 status text NOT NULL CHECK (status IN ('healthy','unhealthy','unknown')),
 checked_at timestamptz NOT NULL DEFAULT now(),
 latency_ms bigint NOT NULL CHECK (latency_ms BETWEEN 0 AND 86400000),
 details text NOT NULL DEFAULT '' CHECK (octet_length(details) <= 2000)
);
CREATE INDEX health_checks_service_idx ON health_checks(service_id,checked_at DESC,id);
