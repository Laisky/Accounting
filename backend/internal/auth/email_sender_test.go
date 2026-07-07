package auth

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSMTPEmailSenderRequiresSTARTTLS verifies SMTP delivery fails closed without STARTTLS.
func TestSMTPEmailSenderRequiresSTARTTLS(t *testing.T) {
	server := startSMTPTestServer(t, false)
	sender := smtpSenderForTest(t, server.addr, false)

	err := sender.SendAuthCode(context.Background(), EmailCodeDelivery{
		Email: "person@example.test",
		Code:  "123456",
	}, EmailCodePurposeVerification)

	require.Error(t, err)
	require.Contains(t, err.Error(), "starttls")
}

// TestSMTPEmailSenderRejectsInvalidTLSCertificate verifies ForceTLSVerify enforces SMTP certificate trust.
func TestSMTPEmailSenderRejectsInvalidTLSCertificate(t *testing.T) {
	server := startSMTPTestServer(t, true)
	sender := smtpSenderForTest(t, server.addr, true)

	err := sender.SendAuthCode(context.Background(), EmailCodeDelivery{
		Email: "person@example.test",
		Code:  "123456",
	}, EmailCodePurposeVerification)

	require.Error(t, err)
	require.Contains(t, err.Error(), "certificate")
}

// TestSMTPEmailSenderSendsWithSTARTTLS verifies SMTP delivery works through explicit STARTTLS.
func TestSMTPEmailSenderSendsWithSTARTTLS(t *testing.T) {
	server := startSMTPTestServer(t, true)
	sender := smtpSenderForTest(t, server.addr, false)

	err := sender.SendAuthCode(context.Background(), EmailCodeDelivery{
		Email: "person@example.test",
		Code:  "123456",
	}, EmailCodePurposePasswordReset)

	require.NoError(t, err)
	require.Eventually(t, func() bool {
		return strings.Contains(server.data(), "Accounting password reset code") &&
			strings.Contains(server.data(), "123456")
	}, time.Second, 10*time.Millisecond)
}

type smtpTestServer struct {
	addr     string
	cert     tls.Certificate
	listener net.Listener
	done     chan struct{}
	dataCh   chan string
}

func startSMTPTestServer(t *testing.T, startTLS bool) *smtpTestServer {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	server := &smtpTestServer{
		addr:     listener.Addr().String(),
		cert:     testCertificate(t),
		listener: listener,
		done:     make(chan struct{}),
		dataCh:   make(chan string, 1),
	}
	go server.serve(startTLS)
	t.Cleanup(func() {
		_ = listener.Close()
		<-server.done
	})

	return server
}

func (s *smtpTestServer) serve(startTLS bool) {
	defer close(s.done)
	conn, err := s.listener.Accept()
	if err != nil {
		return
	}
	defer func() {
		_ = conn.Close()
	}()
	s.handle(conn, startTLS)
}

func (s *smtpTestServer) handle(conn net.Conn, startTLS bool) {
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	writeSMTPLine(writer, "220 smtp.test ESMTP")
	tlsActive := false
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(command, "EHLO"), strings.HasPrefix(command, "HELO"):
			if startTLS && !tlsActive {
				writeSMTPLine(writer, "250-smtp.test")
				writeSMTPLine(writer, "250-STARTTLS")
				writeSMTPLine(writer, "250 OK")
				continue
			}
			writeSMTPLine(writer, "250 OK")
		case command == "STARTTLS":
			writeSMTPLine(writer, "220 Ready to start TLS")
			tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{s.cert}, MinVersion: tls.VersionTLS12})
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			conn = tlsConn
			reader = bufio.NewReader(conn)
			writer = bufio.NewWriter(conn)
			tlsActive = true
		case strings.HasPrefix(command, "MAIL FROM:"):
			writeSMTPLine(writer, "250 OK")
		case strings.HasPrefix(command, "RCPT TO:"):
			writeSMTPLine(writer, "250 OK")
		case command == "DATA":
			writeSMTPLine(writer, "354 End data with <CR><LF>.<CR><LF>")
			var message strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.TrimSpace(dataLine) == "." {
					break
				}
				message.WriteString(dataLine)
			}
			s.dataCh <- message.String()
			writeSMTPLine(writer, "250 OK")
		case command == "QUIT":
			writeSMTPLine(writer, "221 Bye")
			return
		default:
			writeSMTPLine(writer, "250 OK")
		}
	}
}

func (s *smtpTestServer) data() string {
	select {
	case data := <-s.dataCh:
		s.dataCh <- data
		return data
	default:
		return ""
	}
}

func smtpSenderForTest(t *testing.T, addr string, forceVerify bool) *SMTPEmailSender {
	t.Helper()

	host, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)
	cfg := SMTPConfig{
		Host:           host,
		Port:           atoiPort(t, port),
		From:           "accounting@example.test",
		ForceTLSVerify: forceVerify,
	}
	sender, err := NewSMTPEmailSender(cfg)
	require.NoError(t, err)

	return sender
}

func atoiPort(t *testing.T, port string) int {
	t.Helper()

	parsed, ok := new(big.Int).SetString(port, 10)
	require.True(t, ok)
	return int(parsed.Int64())
}

func writeSMTPLine(writer *bufio.Writer, line string) {
	_, _ = writer.WriteString(line + "\r\n")
	_ = writer.Flush()
}

func testCertificate(t *testing.T) tls.Certificate {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "smtp.test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		DNSNames:     []string{"smtp.test"},
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err)

	return cert
}
