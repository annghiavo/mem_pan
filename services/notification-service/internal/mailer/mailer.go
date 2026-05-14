package mailer

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"

	"github.com/jordan-wright/email"
)

type Mailer interface {
	SendWelcome(to, username string) error
	SendEmailVerification(to, username, verifyURL string) error
	SendPasswordReset(to, username, resetURL string) error
}

type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
}

type smtpMailer struct {
	cfg  Config
	addr string
	auth smtp.Auth
}

func New(cfg Config) Mailer {
	return &smtpMailer{
		cfg:  cfg,
		addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		auth: smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host),
	}
}

func (m *smtpMailer) send(to, subject, htmlBody string) error {
	e := email.NewEmail()
	e.From = m.cfg.From
	e.To = []string{to}
	e.Subject = subject
	e.HTML = []byte(htmlBody)
	return e.SendWithTLS(m.addr, m.auth, nil)
}

func (m *smtpMailer) SendWelcome(to, username string) error {
	const tpl = `<!DOCTYPE html><html><body>
<h2>Welcome to MemPan, {{.Username}}!</h2>
<p>Your account has been created. Start building your flashcard decks today.</p>
</body></html>`
	body, err := renderTemplate(tpl, map[string]string{"Username": username})
	if err != nil {
		return err
	}
	return m.send(to, "Welcome to MemPan!", body)
}

func (m *smtpMailer) SendEmailVerification(to, username, verifyURL string) error {
	const tpl = `<!DOCTYPE html><html><body>
<h2>Verify your email, {{.Username}}</h2>
<p>Click the link below to verify your email address. This link expires in 24 hours.</p>
<p><a href="{{.URL}}">Verify Email</a></p>
<p>Or copy this link: {{.URL}}</p>
</body></html>`
	body, err := renderTemplate(tpl, map[string]string{"Username": username, "URL": verifyURL})
	if err != nil {
		return err
	}
	return m.send(to, "Verify your MemPan email", body)
}

func (m *smtpMailer) SendPasswordReset(to, username, resetURL string) error {
	const tpl = `<!DOCTYPE html><html><body>
<h2>Reset your password, {{.Username}}</h2>
<p>We received a request to reset your MemPan password. Click the link below (valid for 1 hour).</p>
<p><a href="{{.URL}}">Reset Password</a></p>
<p>If you did not request this, you can ignore this email.</p>
</body></html>`
	body, err := renderTemplate(tpl, map[string]string{"Username": username, "URL": resetURL})
	if err != nil {
		return err
	}
	return m.send(to, "Reset your MemPan password", body)
}

// noopMailer silently discards all emails (used when SMTP is not configured).
type noopMailer struct{}

func NewNoop() Mailer                                                     { return &noopMailer{} }
func (n *noopMailer) SendWelcome(to, username string) error               { return nil }
func (n *noopMailer) SendEmailVerification(to, username, url string) error { return nil }
func (n *noopMailer) SendPasswordReset(to, username, url string) error    { return nil }

func renderTemplate(tpl string, data any) (string, error) {
	t, err := template.New("").Parse(tpl)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
