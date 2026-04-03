package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestRunRejectsInvalidArgs(t *testing.T) {
	t.Parallel()

	if err := run([]string{"send", "127.0.0.1:1"}, &bytes.Buffer{}); err == nil {
		t.Fatal("run() error = nil, want usage error")
	}
}

func TestSendFromReaderWritesAllBytes(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer ln.Close()

	gotCh := make(chan int, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()

		n, err := io.Copy(io.Discard, conn)
		gotCh <- int(n)
		errCh <- err
	}()

	n, err := sendFromReader(ln.Addr().String(), bytes.NewReader([]byte("hello world")))
	if err != nil {
		t.Fatalf("sendFromReader() error = %v", err)
	}
	if n != int64(len("hello world")) {
		t.Fatalf("sendFromReader() = %d, want %d", n, len("hello world"))
	}
	if err := <-errCh; err != nil {
		t.Fatalf("server copy error = %v", err)
	}
	if got := <-gotCh; got != len("hello world") {
		t.Fatalf("server bytes = %d, want %d", got, len("hello world"))
	}
}

func TestSendTLSWritesAllBytes(t *testing.T) {
	t.Parallel()

	certPEM, keyPEM := mustTestCertificate(t)
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair() error = %v", err)
	}
	ln, err := tls.Listen("tcp4", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS13,
	})
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer ln.Close()

	gotCh := make(chan int64, 1)
	errCh := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errCh <- err
			return
		}
		defer conn.Close()

		n, err := io.Copy(io.Discard, conn)
		gotCh <- n
		errCh <- err
	}()

	n, err := sendTLS(ln.Addr().String(), bytes.NewReader([]byte("hello tls")))
	if err != nil {
		t.Fatalf("sendTLS() error = %v", err)
	}
	if n != int64(len("hello tls")) {
		t.Fatalf("sendTLS() = %d, want %d", n, len("hello tls"))
	}
	if err := <-errCh; err != nil {
		t.Fatalf("server copy error = %v", err)
	}
	if got := <-gotCh; got != int64(len("hello tls")) {
		t.Fatalf("server bytes = %d, want %d", got, len("hello tls"))
	}
}

func TestReceiveFromListenerWritesAllBytes(t *testing.T) {
	t.Parallel()

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() error = %v", err)
	}
	defer ln.Close()

	gotCh := make(chan string, 1)
	errCh := make(chan error, 1)
	go func() {
		var got bytes.Buffer
		n, err := receiveFromListener(ln, &got)
		if err == nil && n != int64(len("hello listener")) {
			err = io.ErrShortWrite
		}
		errCh <- err
		gotCh <- got.String()
	}()

	conn, err := net.Dial("tcp4", ln.Addr().String())
	if err != nil {
		t.Fatalf("Dial() error = %v", err)
	}
	if _, err := conn.Write([]byte("hello listener")); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if err := conn.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := <-errCh; err != nil {
		t.Fatalf("receiveFromListener() error = %v", err)
	}
	if got := <-gotCh; got != "hello listener" {
		t.Fatalf("receiveFromListener() = %q, want %q", got, "hello listener")
	}
}

func mustTestCertificate(t *testing.T) ([]byte, []byte) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey() error = %v", err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate() error = %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return certPEM, keyPEM
}
