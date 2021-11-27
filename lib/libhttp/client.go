package libhttp

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"time"
)

// NewSelfClient returns http client which is independent of settings in os, so may be used inside empty chroot
func NewSelfClient() *http.Client {
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		panic(err)
	}
	dialer := &net.Dialer{
		Resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: time.Millisecond * time.Duration(2000),
				}
				return d.DialContext(ctx, network, "8.8.8.8:53")
			},
		},
	}
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: rootCAs,
			},
			DialContext: dialer.DialContext,
		},
	}
}
