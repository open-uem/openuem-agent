//go:build windows

package report

import (
	"context"
	"time"

	"github.com/yusufpapurcu/wmi"
)

func WMIQueryWithContext(ctx context.Context, query string, dst interface{}, namespace string) error {
	if _, ok := ctx.Deadline(); !ok {
		ctxTimeout, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		ctx = ctxTimeout
	}

	errChan := make(chan error, 1)
	go func() {
		errChan <- wmi.QueryNamespace(query, dst, namespace)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errChan:
		return err
	}
}
