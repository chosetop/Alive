DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM site_settings WHERE default_theme = 'night-ink') THEN
    RAISE EXCEPTION 'cannot remove night-ink theme support while it is the site default';
  END IF;
END $$;

ALTER TABLE site_settings DROP CONSTRAINT site_settings_theme_check;
ALTER TABLE site_settings ADD CONSTRAINT site_settings_theme_check
  CHECK (default_theme IN ('ink', 'lamp', 'codex-lavender'));
