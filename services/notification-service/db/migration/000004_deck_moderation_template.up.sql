INSERT INTO email_templates (template_key, locale, subject, html_body, text_body, variables) VALUES
('deck_moderation', 'en',
 'Important update regarding your MemPan deck',
 E'<!DOCTYPE html><html><body>\n<h2>Hi {{.Username}},</h2>\n<p>Your deck has been {{.DeckStatus}} due to a violation of our policies.</p>\n<p>If you believe this is a mistake, please contact support.</p>\n</body></html>',
 E'Hi {{.Username}},\n\nYour deck has been {{.DeckStatus}} due to a violation of our policies.\n\nIf you believe this is a mistake, please contact support.\n',
 '["Username","DeckStatus"]'::jsonb)
ON CONFLICT (template_key, locale) DO NOTHING;
