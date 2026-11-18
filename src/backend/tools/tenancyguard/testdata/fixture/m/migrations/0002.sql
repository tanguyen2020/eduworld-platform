CREATE TABLE platform.registry (
  tenant_id uuid NOT NULL,
  code varchar(64) NOT NULL,
  PRIMARY KEY (tenant_id)
);
CREATE TABLE platform.registry_inline (tenant_id uuid PRIMARY KEY, code varchar(64));
