INSERT INTO email_templates (template_key, locale, subject, html_body, text_body, variables) VALUES
('report_resolved', 'en',
 'Update on your MemPan report',
 E'<!DOCTYPE html><html><body>\n<h2>Hi {{.Username}},</h2>\n<p>Thanks for helping keep MemPan safe. We''ve finished reviewing the report you submitted.</p>\n<p><strong>Outcome:</strong> {{.Outcome}}</p>\n<p>We appreciate you flagging this. Please continue to report anything that doesn''t belong.</p>\n</body></html>',
 E'Hi {{.Username}},\n\nThanks for helping keep MemPan safe. We''ve finished reviewing the report you submitted.\n\nOutcome: {{.Outcome}}\n\nWe appreciate you flagging this. Please continue to report anything that doesn''t belong.\n',
 '["Username","Outcome"]'::jsonb)
ON CONFLICT (template_key, locale) DO NOTHING;
