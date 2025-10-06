//go:build go1.7
// +build go1.7

// ABOUTME: HTTP helpers for metadata operations on Go 1.7+ runtimes.
// ABOUTME: Extends request execution with IMDS token handling support.
package utils

import (
	"net/http"

	"github.com/rexray/rexray/libstorage/api/types"
)

func doRequest(ctx types.Context, req *http.Request) (*http.Response, error) {
	return doRequestWithClient(ctx, http.DefaultClient, req)
}

func doRequestWithClient(
	ctx types.Context,
	client *http.Client,
	req *http.Request) (*http.Response, error) {
	if err := maybeAttachIMDSToken(ctx, client, req); err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	return client.Do(req)
}
