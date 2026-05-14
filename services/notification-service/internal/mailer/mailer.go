package mailer

import (
	"bytes"
	"crypto/tls"
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

func (m *smtpMailer) send(to, subject, textBody, htmlBody string) error {
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

func (m *smtpMailer) SendWelcome(to, username string) error {
	const htmlTpl = `<!DOCTYPE html><html><body>
<h2>Welcome to MemPan, {{.Username}}!</h2>
<p>Your account has been created. Start building your flashcard decks today.</p>
</body></html>`
	const textTpl = `Welcome to MemPan, {{.Username}}!

Your account has been created. Start building your flashcard decks today.
`
	data := map[string]string{"Username": username}
	htmlBody, err := renderTemplate(htmlTpl, data)
	if err != nil {
		return err
	}
	textBody, err := renderTemplate(textTpl, data)
	if err != nil {
		return err
	}
	return m.send(to, "Welcome to MemPan!", textBody, htmlBody)
}

func (m *smtpMailer) SendEmailVerification(to, username, verifyURL string) error {
	const htmlTpl = `<!DOCTYPE html><html><body>
<h2>Verify your email, {{.Username}}</h2>
<p>Click the link below to verify your email address. This link expires in 24 hours.</p>
<p><a href="{{.URL}}">Verify Email</a></p>
<p>Or copy this link: {{.URL}}</p>
</body></html>`
	const textTpl = `Verify your email, {{.Username}}

Open this link to verify your email address (expires in 24 hours):
{{.URL}}
`
	data := map[string]string{"Username": username, "URL": verifyURL}
	htmlBody, err := renderTemplate(htmlTpl, data)
	if err != nil {
		return err
	}
	textBody, err := renderTemplate(textTpl, data)
	if err != nil {
		return err
	}
	return m.send(to, "Verify your MemPan email", textBody, htmlBody)
}

func (m *smtpMailer) SendPasswordReset(to, username, resetURL string) error {
	const htmlTpl = `<!DOCTYPE html><html><body>
<h2>Reset your password, {{.Username}}</h2>
<p>We received a request to reset your MemPan password. Click the link below (valid for 1 hour).</p>
<p><a href="{{.URL}}">Reset Password</a></p>
<p>If you did not request this, you can ignore this email.</p>
</body></html>`
	const textTpl = `Reset your password, {{.Username}}

We received a request to reset your MemPan password. Open this link (valid for 1 hour):
{{.URL}}

If you did not request this, you can ignore this email.
`
	data := map[string]string{"Username": username, "URL": resetURL}
	htmlBody, err := renderTemplate(htmlTpl, data)
	if err != nil {
		return err
	}
	textBody, err := renderTemplate(textTpl, data)
	if err != nil {
		return err
	}
	return m.send(to, "Reset your MemPan password", textBody, htmlBody)
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
