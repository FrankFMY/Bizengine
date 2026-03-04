-- 007_logistics: Logistics tables (routes, stops, geo tracks)

CREATE TABLE routes (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id  UUID NOT NULL,
    name          TEXT NOT NULL,
    vehicle_id    UUID REFERENCES entities(id),
    driver_id     UUID REFERENCES entities(id),
    status        TEXT NOT NULL DEFAULT 'planned',
    planned_start TIMESTAMPTZ,
    planned_end   TIMESTAMPTZ,
    actual_start  TIMESTAMPTZ,
    actual_end    TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_routes_ws ON routes(workspace_id, status);
ALTER TABLE routes ENABLE ROW LEVEL SECURITY;

CREATE TABLE route_stops (
    id              UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    route_id        UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
    workspace_id    UUID NOT NULL,
    location_id     UUID REFERENCES entities(id),
    address         TEXT NOT NULL DEFAULT '',
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,
    sort_order      INT NOT NULL DEFAULT 0,
    planned_arrival TIMESTAMPTZ,
    actual_arrival  TIMESTAMPTZ,
    status          TEXT NOT NULL DEFAULT 'pending',
    delivery_ids    UUID[] DEFAULT '{}',
    notes           TEXT NOT NULL DEFAULT ''
);
CREATE INDEX idx_rs_route ON route_stops(route_id, sort_order);
ALTER TABLE route_stops ENABLE ROW LEVEL SECURITY;

CREATE TABLE geo_tracks (
    workspace_id UUID NOT NULL,
    entity_id    UUID NOT NULL,
    recorded_at  TIMESTAMPTZ NOT NULL,
    latitude     DOUBLE PRECISION NOT NULL,
    longitude    DOUBLE PRECISION NOT NULL,
    speed        DOUBLE PRECISION DEFAULT 0,
    heading      DOUBLE PRECISION DEFAULT 0,
    PRIMARY KEY (entity_id, recorded_at)
) PARTITION BY RANGE (recorded_at);

CREATE TABLE geo_tracks_default PARTITION OF geo_tracks DEFAULT;
CREATE INDEX idx_gt_ws_entity ON geo_tracks(workspace_id, entity_id, recorded_at DESC);
