ALTER TABLE site_settings DROP CONSTRAINT site_settings_theme_check;
ALTER TABLE site_settings ADD CONSTRAINT site_settings_theme_check
  CHECK (default_theme IN ('ink', 'lamp', 'codex-lavender', 'night-ink'));
