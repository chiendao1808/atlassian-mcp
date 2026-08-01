package transport

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"

	"github.com/chiendao1808/atlassian-mcp/internal/config"
)

func NewHTTPClient(shared config.Shared, caFile string) (*http.Client, error) {
	tlsConfig := &tls.Config{InsecureSkipVerify: !shared.TLSVerify} //nolint:gosec
	if shared.TLSVerify && caFile != "" {
		pem, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil || pool == nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("CA file contains no PEM certificates")
		}
		tlsConfig.RootCAs = pool
	}
	return &http.Client{
		Timeout: shared.RequestTimeout,
		Transport: &http.Transport{
			TLSClientConfig:     tlsConfig,
			TLSHandshakeTimeout: shared.ConnectTimeout,
		},
	}, nil
}
