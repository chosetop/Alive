CREATE TABLE site_settings (
  id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
  default_theme VARCHAR(64) NOT NULL DEFAULT 'ink',
  revision BIGINT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT site_settings_theme_check
    CHECK (default_theme IN ('ink', 'lamp', 'codex-lavender'))
);

INSERT INTO site_settings (id) VALUES (1);

CREATE TRIGGER site_settings_set_updated_at
  BEFORE UPDATE ON site_settings
  FOR EACH ROW EXECUTE FUNCTION set_updated_at();
