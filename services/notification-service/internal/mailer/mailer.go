package mailer

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	htmltpl "html/template"
	"log"
	"net/smtp"
	"sync"
	"text/template"
	"time"

	"github.com/jordan-wright/email"
)

const (
	KeyWelcome            = "welcome"
	KeyEmailVerification  = "email_verification"
	KeyPasswordReset      = "password_reset"
	KeyStudyReminder      = "study_reminder"
	KeyReportResolved     = "report_resolved"
	KeyDeckModeration     = "deck_moderation"
	DefaultLocale         = "en"
	defaultCacheTTL       = 60 * time.Second
)

// Template is a renderable email template loaded from storage.
type Template struct {
	Subject  string
	HTML     string
	Text     string
}

// TemplateStore returns the active template for a key/locale pair.
// Implementations should be safe for concurrent use.
type TemplateStore interface {
	Get(ctx context.Context, key, locale string) (Template, error)
}

// Mailer renders and sends transactional emails.
type Mailer interface {
	// Send renders the named template with the supplied data and emails it.
	Send(ctx context.Context, to, key string, data any) error

	// SendRaw bypasses the template store. Useful for admin test sends
	// where the caller has already-rendered content.
	SendRaw(ctx context.Context, to, subject, htmlBody, textBody string) error

	// Convenience wrappers used by event handlers.
	SendWelcome(ctx context.Context, to, username string) error
	SendEmailVerification(ctx context.Context, to, username, verifyURL string) error
	SendPasswordReset(ctx context.Context, to, username, resetURL string) error
	SendReportResolved(ctx context.Context, to, username, outcome string) error
	SendDeckModeration(ctx context.Context, to, username, deckStatus string) error
}

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type smtpMailer struct {
	cfg   Config
	addr  string
	auth  smtp.Auth
	store TemplateStore
}

func New(cfg Config, store TemplateStore) Mailer {
	return &smtpMailer{
		cfg:   cfg,
		addr:  fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		auth:  smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host),
		store: store,
	}
}

func (m *smtpMailer) Send(ctx context.Context, to, key string, data any) error {
	tpl, err := m.store.Get(ctx, key, DefaultLocale)
	if err != nil {
		log.Printf("[mailer] template %q lookup failed (%v) — falling back to embedded default", key, err)
		tpl, err = defaultTemplate(key)
		if err != nil {
			return err
		}
	}
	subject, htmlBody, textBody, err := Render(tpl, data)
	if err != nil {
		return fmt.Errorf("render %q: %w", key, err)
	}
	return m.SendRaw(ctx, to, subject, htmlBody, textBody)
}

func (m *smtpMailer) SendRaw(_ context.Context, to, subject, htmlBody, textBody string) error {
	e := email.NewEmail()
	e.From = m.cfg.From
	e.To = []string{to}
	e.ReplyTo = []string{m.cfg.From}
	e.Subject = subject
	e.Text = []byte(textBody)
	e.HTML = []byte(htmlBody)
	tlsCfg := &tls.Config{ServerName: m.cfg.Host}
	if m.cfg.Port == 465 {
		return e.SendWithTLS(m.addr, m.auth, tlsCfg)
	}
	return e.SendWithStartTLS(m.addr, m.auth, tlsCfg)
}

func (m *smtpMailer) SendWelcome(ctx context.Context, to, username string) error {
	return m.Send(ctx, to, KeyWelcome, map[string]string{"Username": username})
}

func (m *smtpMailer) SendEmailVerification(ctx context.Context, to, username, verifyURL string) error {
	return m.Send(ctx, to, KeyEmailVerification, map[string]string{"Username": username, "URL": verifyURL})
}

func (m *smtpMailer) SendPasswordReset(ctx context.Context, to, username, resetURL string) error {
	return m.Send(ctx, to, KeyPasswordReset, map[string]string{"Username": username, "URL": resetURL})
}

func (m *smtpMailer) SendReportResolved(ctx context.Context, to, username, outcome string) error {
	return m.Send(ctx, to, KeyReportResolved, map[string]string{"Username": username, "Outcome": outcome})
}

func (m *smtpMailer) SendDeckModeration(ctx context.Context, to, username, deckStatus string) error {
	return m.Send(ctx, to, KeyDeckModeration, map[string]string{"Username": username, "DeckStatus": deckStatus})
}

// noopMailer silently discards all emails (used when SMTP is not configured).
type noopMailer struct{}

func NewNoop() Mailer { return &noopMailer{} }

func (n *noopMailer) Send(context.Context, string, string, any) error                  { return nil }
func (n *noopMailer) SendRaw(context.Context, string, string, string, string) error    { return nil }
func (n *noopMailer) SendWelcome(context.Context, string, string) error                { return nil }
func (n *noopMailer) SendEmailVerification(context.Context, string, string, string) error { return nil }
func (n *noopMailer) SendPasswordReset(context.Context, string, string, string) error  { return nil }
func (n *noopMailer) SendReportResolved(context.Context, string, string, string) error { return nil }
func (n *noopMailer) SendDeckModeration(context.Context, string, string, string) error { return nil }

// Render executes a Template against data and returns the rendered triple.
// HTML body uses html/template (auto-escaping); subject and text body use text/template.
func Render(t Template, data any) (subject, htmlBody, textBody string, err error) {
	subject, err = renderText(t.Subject, data)
	if err != nil {
		return "", "", "", fmt.Errorf("subject: %w", err)
	}
	htmlBody, err = renderHTML(t.HTML, data)
	if err != nil {
		return "", "", "", fmt.Errorf("html: %w", err)
	}
	textBody, err = renderText(t.Text, data)
	if err != nil {
		return "", "", "", fmt.Errorf("text: %w", err)
	}
	return subject, htmlBody, textBody, nil
}

func renderText(tpl string, data any) (string, error) {
	parsed, err := template.New("").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := parsed.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderHTML(tpl string, data any) (string, error) {
	parsed, err := htmltpl.New("").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := parsed.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// ValidateTemplate parses each section to surface syntax errors before save.
func ValidateTemplate(t Template) error {
	if _, err := template.New("").Parse(t.Subject); err != nil {
		return fmt.Errorf("subject parse: %w", err)
	}
	if _, err := htmltpl.New("").Parse(t.HTML); err != nil {
		return fmt.Errorf("html parse: %w", err)
	}
	if _, err := template.New("").Parse(t.Text); err != nil {
		return fmt.Errorf("text parse: %w", err)
	}
	return nil
}

// ---------- TemplateStore implementations ----------

// CachedStore wraps a base store with a TTL cache, keyed by (key, locale).
type CachedStore struct {
	base TemplateStore
	ttl  time.Duration

	mu      sync.RWMutex
	entries map[string]cacheEntry
}

type cacheEntry struct {
	tpl       Template
	expiresAt time.Time
}

func NewCachedStore(base TemplateStore) *CachedStore {
	return &CachedStore{base: base, ttl: defaultCacheTTL, entries: make(map[string]cacheEntry)}
}

func (c *CachedStore) Get(ctx context.Context, key, locale string) (Template, error) {
	cacheKey := key + "|" + locale
	c.mu.RLock()
	if e, ok := c.entries[cacheKey]; ok && time.Now().Before(e.expiresAt) {
		c.mu.RUnlock()
		return e.tpl, nil
	}
	c.mu.RUnlock()

	tpl, err := c.base.Get(ctx, key, locale)
	if err != nil {
		return Template{}, err
	}
	c.mu.Lock()
	c.entries[cacheKey] = cacheEntry{tpl: tpl, expiresAt: time.Now().Add(c.ttl)}
	c.mu.Unlock()
	return tpl, nil
}

// Invalidate forces the next Get to bypass the cache for the given key.
func (c *CachedStore) Invalidate(key, locale string) {
	c.mu.Lock()
	delete(c.entries, key+"|"+locale)
	c.mu.Unlock()
}

// ---------- Embedded fallbacks ----------

var defaultTemplates = map[string]Template{
	KeyWelcome: {
		Subject: "Welcome to MemPan!",
		HTML:    "<!DOCTYPE html><html><body>\n<h2>Welcome to MemPan, {{.Username}}!</h2>\n<p>Your account has been created. Start building your flashcard decks today.</p>\n</body></html>",
		Text:    "Welcome to MemPan, {{.Username}}!\n\nYour account has been created. Start building your flashcard decks today.\n",
	},
	KeyEmailVerification: {
		Subject: "Verify your MemPan email",
		HTML:    "<!DOCTYPE html><html><body>\n<h2>Verify your email, {{.Username}}</h2>\n<p>Click the link below to verify your email address. This link expires in 24 hours.</p>\n<p><a href=\"{{.URL}}\">Verify Email</a></p>\n<p>Or copy this link: {{.URL}}</p>\n</body></html>",
		Text:    "Verify your email, {{.Username}}\n\nOpen this link to verify your email address (expires in 24 hours):\n{{.URL}}\n",
	},
	KeyPasswordReset: {
		Subject: "Reset your MemPan password",
		HTML:    "<!DOCTYPE html><html><body>\n<h2>Reset your password, {{.Username}}</h2>\n<p>We received a request to reset your MemPan password. Click the link below (valid for 1 hour).</p>\n<p><a href=\"{{.URL}}\">Reset Password</a></p>\n<p>If you did not request this, you can ignore this email.</p>\n</body></html>",
		Text:    "Reset your password, {{.Username}}\n\nWe received a request to reset your MemPan password. Open this link (valid for 1 hour):\n{{.URL}}\n\nIf you did not request this, you can ignore this email.\n",
	},
	KeyStudyReminder: {
		Subject: "Your MemPan study session is waiting",
		HTML:    "<!DOCTYPE html><html><body>\n<h2>Time to study, {{.Username}}!</h2>\n<p>You have {{.DueCount}} card(s) due for review today. Keep your streak alive.</p>\n<p><a href=\"{{.URL}}\">Open MemPan</a></p>\n</body></html>",
		Text:    "Time to study, {{.Username}}!\n\nYou have {{.DueCount}} card(s) due for review today. Keep your streak alive.\n{{.URL}}\n",
	},
	KeyReportResolved: {
		Subject: "Update on your MemPan report",
		HTML:    "<!DOCTYPE html><html><body>\n<h2>Hi {{.Username}},</h2>\n<p>Thanks for helping keep MemPan safe. We've finished reviewing the report you submitted.</p>\n<p><strong>Outcome:</strong> {{.Outcome}}</p>\n<p>We appreciate you flagging this. Please continue to report anything that doesn't belong.</p>\n</body></html>",
		Text:    "Hi {{.Username}},\n\nThanks for helping keep MemPan safe. We've finished reviewing the report you submitted.\n\nOutcome: {{.Outcome}}\n\nWe appreciate you flagging this. Please continue to report anything that doesn't belong.\n",
	},
	KeyDeckModeration: {
		Subject: "Important update regarding your MemPan deck",
		HTML:    "<!DOCTYPE html><html><body>\n<h2>Hi {{.Username}},</h2>\n<p>Your deck has been {{.DeckStatus}} due to a violation of our policies.</p>\n<p>If you believe this is a mistake, please contact support.</p>\n</body></html>",
		Text:    "Hi {{.Username}},\n\nYour deck has been {{.DeckStatus}} due to a violation of our policies.\n\nIf you believe this is a mistake, please contact support.\n",
	},
}

func defaultTemplate(key string) (Template, error) {
	t, ok := defaultTemplates[key]
	if !ok {
		return Template{}, fmt.Errorf("no template for key %q", key)
	}
	return t, nil
}
