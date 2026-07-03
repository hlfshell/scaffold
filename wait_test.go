package scaffold

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestWaitFunc(t *testing.T) {
	attempts := 0

	err := WaitFunc(context.Background(), 1*time.Second, 50*time.Millisecond, func(ctx context.Context) error {
		attempts++
		if attempts < 2 {
			return errNotReady{}
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitFuncCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := WaitFunc(ctx, time.Second, time.Millisecond, func(ctx context.Context) error {
		return errNotReady{}
	})
	if !errors.Is(err, errNotReady{}) {
		t.Fatalf("expected last error, got %v", err)
	}
}

func TestWaitForTCP(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	address := listener.Addr().(*net.TCPAddr)

	err = WaitForTCP(context.Background(), "127.0.0.1", fmtPort(address.Port), 1*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitForHTTP(t *testing.T) {
	server := &http.Server{
		Addr: "127.0.0.1:0",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}),
	}

	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	go server.Serve(listener)
	defer server.Close()

	err = WaitForHTTP(context.Background(), "http://"+listener.Addr().String(), http.StatusNoContent, 1*time.Second)
	if err != nil {
		t.Fatal(err)
	}
}

type errNotReady struct{}

func (e errNotReady) Error() string {
	return "not ready"
}

func fmtPort(port int) string {
	return fmt.Sprintf("%d", port)
}

func TestWaitForLogTextReturnsEOFWithoutMatch(t *testing.T) {
	err := WaitForLogText(context.Background(), strings.NewReader("hello\n"), "ready", time.Second)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF, got %v", err)
	}
}
