CREATE TABLE tenants (
    id uuid PRIMARY KEY DEFAULT uuidv7(),
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE machines (
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    id uuid NOT NULL DEFAULT uuidv7(),
    machine_code text NOT NULL,
    name text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, machine_code)
);

CREATE TABLE products (
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    id uuid NOT NULL DEFAULT uuidv7(),
    sku text NOT NULL,
    name text NOT NULL,
    unit_cost numeric(12, 4) NOT NULL CHECK (unit_cost >= 0),
    currency char(3) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    UNIQUE (tenant_id, sku)
);

CREATE TABLE inspection_results (
    tenant_id uuid NOT NULL REFERENCES tenants(id),
    id uuid NOT NULL DEFAULT uuidv7(),
    machine_id uuid NOT NULL,
    product_id uuid NOT NULL,
    outcome text NOT NULL CHECK (outcome IN ('finished', 'deformed')),
    inspected_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id),
    FOREIGN KEY (tenant_id, machine_id) REFERENCES machines (tenant_id, id),
    FOREIGN KEY (tenant_id, product_id) REFERENCES products (tenant_id, id)
);

CREATE INDEX inspection_results_machine_time ON inspection_results (tenant_id, machine_id, inspected_at);

CREATE INDEX inspection_results_product_time ON inspection_results (tenant_id, product_id, inspected_at);