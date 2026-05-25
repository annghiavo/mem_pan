-- Seed the two appeal-related email templates so they can be customised from
-- the admin UI without redeploying the service.

INSERT INTO email_templates (template_key, locale, subject, html_body, text_body, variables) VALUES
('deck_deleted_with_appeal', 'en',
 'Your MemPan deck "{{.DeckName}}" was removed — you can appeal',
 E'<!DOCTYPE html><html><body>\n<h2>Hi {{.Username}},</h2>\n<p>Your deck <strong>"{{.DeckName}}"</strong> has been <strong>removed</strong> from MemPan.</p>\n<p><strong>Reason:</strong> {{.Reason}}</p>\n<p>If you believe this is a mistake, you can appeal this decision. A moderator will review your case and email you with the final outcome.</p>\n<p style="margin:24px 0;"><a href="{{.AppealURL}}" style="background:#2563eb;color:white;padding:10px 18px;border-radius:6px;text-decoration:none;font-weight:600;">Submit an appeal</a></p>\n<p style="font-size:13px;color:#666;">Or copy this link into your browser:<br/>{{.AppealURL}}</p>\n<p style="font-size:13px;color:#666;">This link is unique to your deck and can only be used to file one appeal.</p>\n</body></html>',
 E'Hi {{.Username}},\n\nYour deck "{{.DeckName}}" has been removed from MemPan.\n\nReason: {{.Reason}}\n\nIf you believe this is a mistake, you can appeal this decision. A moderator will review your case and email you with the final outcome.\n\nSubmit an appeal:\n{{.AppealURL}}\n\nThis link is unique to your deck and can only be used to file one appeal.\n',
 '["Username","DeckName","Reason","AppealURL"]'::jsonb)
ON CONFLICT (template_key, locale) DO NOTHING;

INSERT INTO email_templates (template_key, locale, subject, html_body, text_body, variables) VALUES
('appeal_decided', 'en',
 'Decision on your MemPan deck appeal for "{{.DeckName}}"',
 E'<!DOCTYPE html><html><body>\n<h2>Hi {{.Username}},</h2>\n<p>A moderator has reviewed your appeal for the deck <strong>"{{.DeckName}}"</strong>.</p>\n<p><strong>Decision:</strong> {{.Outcome}}.</p>\n{{if .DecisionNote}}<p><strong>Moderator note:</strong> {{.DecisionNote}}</p>{{end}}\n<p>This appeal is now closed. No further action is required.</p>\n</body></html>',
 E'Hi {{.Username}},\n\nA moderator has reviewed your appeal for the deck "{{.DeckName}}".\n\nDecision: {{.Outcome}}.\n{{if .DecisionNote}}Moderator note: {{.DecisionNote}}\n{{end}}\nThis appeal is now closed. No further action is required.\n',
 '["Username","DeckName","Decision","Outcome","DecisionNote"]'::jsonb)
ON CONFLICT (template_key, locale) DO NOTHING;
