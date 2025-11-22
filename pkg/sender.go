package email_sender

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/wb-go/wbf/zlog"

	"DelayedNotifier/internal/domain"
)

type SMTPSender struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	Timeout  time.Duration
}

func NewSMTPSender(host string, port int, username, password, from string) *SMTPSender {
	return &SMTPSender{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
		Timeout:  10 * time.Second,
	}
}

func (s *SMTPSender) Send(ctx context.Context, n *domain.Notification) error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	auth := smtp.PlainAuth("SSL", s.Username, s.Password, s.Host)

	contentType := "text/html; charset=utf-8"

	var subject, body string

	if sub, ok := n.Payload["subject"]; ok {
		subject = sub.(string)
	}

	if b, ok := n.Payload["body"]; ok {
		body = b.(string)
	} else {
		var parts []string
		for k, v := range n.Payload {
			parts = append(parts, fmt.Sprintf("%s=%v", k, v))
		}
		body = strings.Join(parts, ", ")
	}

	msg := []byte(fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: %s\r\n\r\n%s",
		s.From,
		n.Recipient,
		subject,
		contentType,
		body,
	))

	done := make(chan error, 1)
	go func() {
		err := smtp.SendMail(addr, auth, s.From, []string{n.Recipient}, msg)
		zlog.Logger.Err(err).Msg("send email")
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
