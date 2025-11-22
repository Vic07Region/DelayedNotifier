package email_sender

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"sync"
	"time"

	"DelayedNotifier/internal/domain"
)

// SMTPSender структура для отправки email через SMTP.
type SMTPSender struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	SSL      bool

	Timeout time.Duration

	mu     sync.Mutex
	client *smtp.Client
}

// NewSMTPSender создает новый экземпляр SMTPSender.
func NewSMTPSender(host string, port int, username, password, from string, ssl bool) (*SMTPSender, error) {
	s := &SMTPSender{
		Host:     host,
		Port:     port,
		Username: username,
		Password: password,
		From:     from,
		SSL:      ssl,
		Timeout:  10 * time.Second,
	}

	if err := s.connect(); err != nil {
		return nil, err
	}

	return s, nil
}

// connect устанавливает соединение с SMTP сервером.
func (s *SMTPSender) connect() error {
	addr := fmt.Sprintf("%s:%d", s.Host, s.Port)
	dialer := &net.Dialer{Timeout: s.Timeout}

	var conn net.Conn
	var err error

	if s.SSL {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: s.Host})
	} else {
		conn, err = dialer.Dial("tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}

	clientChan := make(chan *smtp.Client, 1)
	errChan := make(chan error, 1)

	go func() {
		client, err := smtp.NewClient(conn, s.Host)
		if err != nil {
			_ = conn.Close()
			errChan <- err
			return
		}
		clientChan <- client
	}()

	var client *smtp.Client
	select {
	case client = <-clientChan:
	case err := <-errChan:
		return fmt.Errorf("smtp.NewClient failed: %w", err)
	case <-time.After(s.Timeout):
		_ = conn.Close()
		return fmt.Errorf("smtp.NewClient timed out (server did not send banner)")
	}

	if !s.SSL {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(&tls.Config{ServerName: s.Host}); err != nil {
				fmt.Printf("Warning: STARTTLS not available: %v\n", err)
			}
		}
	}

	// Пропускаем аутентификацию, если credentials пустые (для MailHog и других тестовых SMTP)
	if s.Username == "" && s.Password == "" {
		fmt.Printf("Note: Skipping authentication (empty credentials for MailHog)\n")
	} else {
		// Проверяем, поддерживает ли сервер аутентификацию
		auth := smtp.CRAMMD5Auth(s.Username, s.Password)

		//auth := smtp.PlainAuth("delayed_notifier", s.Username, s.Password, s.Host)

		if ok, _ := client.Extension("AUTH"); ok {

			if err := client.Auth(auth); err != nil {
				_ = client.Close()
				return fmt.Errorf("authentication failed: %w", err)
			}
		} else {
			fmt.Printf("Note: SMTP server does not support authentication, continuing without auth\n")
		}
	}

	s.client = client
	if err := s.ensureConnected(); err != nil {
		_ = client.Close()
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	return nil
}

// ensureConnected проверяет и восстанавливает соединение с SMTP сервером.
func (s *SMTPSender) ensureConnected() error {
	if s.client != nil {
		if err := s.client.Noop(); err == nil {
			return nil
		}
	}
	return s.connect()
}

// Send отправляет email уведомление.
func (s *SMTPSender) Send(ctx context.Context, n *domain.Notification) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureConnected(); err != nil {
		return err
	}

	contentType := "text/html; charset=utf-8"

	subject := ""
	if v, ok := n.Payload["subject"]; ok {
		subject = v.(string)
	}

	body := ""
	if v, ok := n.Payload["body"]; ok {
		body = v.(string)
	} else {
		parts := make([]string, 0, len(n.Payload))
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
		err := s.sendMessage(n.Recipient, msg)
		done <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

// sendMessage отправляет сообщение через установленное SMTP соединение.
func (s *SMTPSender) sendMessage(recipient string, msg []byte) error {
	if err := s.client.Mail(s.From); err != nil {
		return err
	}
	if err := s.client.Rcpt(recipient); err != nil {
		return err
	}
	w, err := s.client.Data()
	if err != nil {
		return err
	}
	if _, err = w.Write(msg); err != nil {
		return err
	}

	return w.Close()
}

// Close закрывает SMTP соединение.
func (s *SMTPSender) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client != nil {
		_ = s.client.Quit()
		s.client = nil
	}

	return nil
}
