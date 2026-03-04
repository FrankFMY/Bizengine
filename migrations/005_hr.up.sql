-- 005_hr: HR tables (shifts, timesheets)

CREATE TABLE shifts (
    id            UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id  UUID NOT NULL,
    employee_id   UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    location_id   UUID REFERENCES entities(id),
    start_time    TIMESTAMPTZ NOT NULL,
    end_time      TIMESTAMPTZ NOT NULL,
    break_minutes INT NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'scheduled',
    notes         TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_shifts_employee ON shifts(workspace_id, employee_id, start_time);
CREATE INDEX idx_shifts_location ON shifts(workspace_id, location_id, start_time);
CREATE INDEX idx_shifts_date ON shifts(workspace_id, start_time DESC);
ALTER TABLE shifts ENABLE ROW LEVEL SECURITY;

CREATE TABLE timesheets (
    id           UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL,
    employee_id  UUID NOT NULL REFERENCES entities(id) ON DELETE CASCADE,
    shift_id     UUID REFERENCES shifts(id),
    clock_in     TIMESTAMPTZ NOT NULL,
    clock_out    TIMESTAMPTZ,
    hours_worked NUMERIC(5,2),
    status       TEXT NOT NULL DEFAULT 'open',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_ts_employee ON timesheets(workspace_id, employee_id, clock_in DESC);
ALTER TABLE timesheets ENABLE ROW LEVEL SECURITY;
