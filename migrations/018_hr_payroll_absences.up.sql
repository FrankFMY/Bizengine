-- Payroll records
CREATE TABLE IF NOT EXISTS payrolls (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    employee_id     UUID NOT NULL,
    year            INT NOT NULL,
    month           INT NOT NULL CHECK (month >= 1 AND month <= 12),
    gross_salary    BIGINT NOT NULL,
    ndfl            BIGINT NOT NULL DEFAULT 0,
    deductions      BIGINT NOT NULL DEFAULT 0,
    net_salary      BIGINT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'draft',
    approved_at     TIMESTAMPTZ,
    approved_by     UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_payrolls_org ON payrolls(organization_id);
CREATE INDEX idx_payrolls_employee ON payrolls(organization_id, employee_id);
CREATE UNIQUE INDEX idx_payrolls_unique_period ON payrolls(organization_id, employee_id, year, month);

-- Absence records
CREATE TABLE IF NOT EXISTS absences (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_id UUID NOT NULL REFERENCES organizations(id),
    employee_id     UUID NOT NULL,
    type            TEXT NOT NULL CHECK (type IN ('vacation', 'sick_leave', 'personal', 'unpaid')),
    start_date      DATE NOT NULL,
    end_date        DATE NOT NULL,
    days            INT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending',
    notes           TEXT NOT NULL DEFAULT '',
    approved_by     UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_absences_org ON absences(organization_id);
CREATE INDEX idx_absences_employee ON absences(organization_id, employee_id);
CREATE INDEX idx_absences_dates ON absences(organization_id, start_date, end_date);
