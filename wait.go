package scaffold

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

/*
WaitForTCP waits until a TCP connection can be opened to host:port, or
until the timeout is reached.
*/
func WaitForTCP(ctx context.Context, host string, port string, timeout time.Duration) error {
	address := net.JoinHostPort(host, port)

	return WaitFunc(ctx, timeout, 50*time.Millisecond, func(ctx context.Context) error {
		dialer := &net.Dialer{
			Timeout: 500 * time.Millisecond,
		}

		connection, err := dialer.DialContext(ctx, "tcp", address)
		if err != nil {
			return err
		}
		defer connection.Close()

		return nil
	})
}

/*
WaitForHTTP waits until the URL returns the expected HTTP status code, or
until the timeout is reached.
*/
func WaitForHTTP(ctx context.Context, url string, statusCode int, timeout time.Duration) error {
	client := &http.Client{
		Timeout: 1 * time.Second,
	}

	return WaitFunc(ctx, timeout, 50*time.Millisecond, func(ctx context.Context) error {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return err
		}

		response, err := client.Do(request)
		if err != nil {
			return err
		}
		defer response.Body.Close()

		if response.StatusCode != statusCode {
			return fmt.Errorf("unexpected status code %d", response.StatusCode)
		}

		return nil
	})
}

/*
WaitForLogText scans a reader until the requested text appears. This is
useful for containers that only advertise readiness through logs.
*/
func WaitForLogText(ctx context.Context, reader io.Reader, text string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		scanner := bufio.NewScanner(reader)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)

		for scanner.Scan() {
			if strings.Contains(scanner.Text(), text) {
				done <- nil
				return
			}
		}

		if err := scanner.Err(); err != nil {
			done <- err
			return
		}

		done <- io.EOF
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("timed out waiting for log text %q", text)
	}
}

/*
WaitFunc retries a function until it returns nil or the timeout is
reached. The last error is wrapped into the timeout error.
*/
func WaitFunc(ctx context.Context, timeout time.Duration, interval time.Duration, fn func(context.Context) error) error {
	if interval <= 0 {
		interval = 50 * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var lastErr error

	for {
		err := fn(ctx)
		if err == nil {
			return nil
		}

		lastErr = err

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			if lastErr != nil {
				return fmt.Errorf("timed out waiting: %w", lastErr)
			}

			return fmt.Errorf("timed out waiting: %w", ctx.Err())
		case <-timer.C:
		}
	}
}
