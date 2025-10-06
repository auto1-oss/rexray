package utils

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rexray/rexray/libstorage/api/context"

	"github.com/rexray/rexray/libstorage/drivers/storage/ebs"
)

func skipTest(t *testing.T) {
	if ok, _ := strconv.ParseBool(os.Getenv("EBS_UTILS_TEST")); !ok {
		t.Skip()
	}
}

func TestInstanceID(t *testing.T) {
	skipTest(t)
	iid, err := InstanceID(context.Background(), ebs.Name)
	if !assert.NoError(t, err) {
		t.FailNow()
	}
	t.Logf("instanceID=%s", iid.String())
}

func TestInstanceIDSupportsIMDSv2(t *testing.T) {
	withMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest/api/token":
			if r.Method != http.MethodPut {
				t.Fatalf("unexpected token method: %s", r.Method)
			}
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "token-abc")
		case "/latest/dynamic/instance-identity/document":
			if r.Header.Get(tokenHeader) != "token-abc" {
				t.Fatalf("missing metadata token header")
			}
			io.WriteString(w, `{"instanceId":"i-abc","region":"us-west-2","availabilityZone":"us-west-2a"}`)
		default:
			http.NotFound(w, r)
		}
	}, func() {
		iid, err := InstanceID(context.Background(), ebs.Name)
		require.NoError(t, err)
		require.Equal(t, "i-abc", iid.ID)
		require.Equal(t, "us-west-2", iid.Fields[ebs.InstanceIDFieldRegion])
	})
}

func TestInstanceIDFallsBackWithoutTokenSupport(t *testing.T) {
	var tokenCalls int
	withMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest/api/token":
			tokenCalls++
			w.WriteHeader(http.StatusNotFound)
		case "/latest/dynamic/instance-identity/document":
			if r.Header.Get(tokenHeader) != "" {
				t.Fatalf("expected request without metadata token header")
			}
			io.WriteString(w, `{"instanceId":"i-def","region":"us-east-1","availabilityZone":"us-east-1a"}`)
		default:
			http.NotFound(w, r)
		}
	}, func() {
		iid, err := InstanceID(context.Background(), ebs.Name)
		require.NoError(t, err)
		require.Equal(t, "i-def", iid.ID)
		require.Equal(t, "us-east-1", iid.Fields[ebs.InstanceIDFieldRegion])
		require.Equal(t, 1, tokenCalls)
	})
}

func TestIMDSRoleCredentials(t *testing.T) {
	withMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest/api/token":
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "token-xyz")
		case "/latest/meta-data/iam/security-credentials/":
			if r.Header.Get(tokenHeader) != "token-xyz" {
				t.Fatalf("expected token header for role list")
			}
			io.WriteString(w, "role-one\n")
		case "/latest/meta-data/iam/security-credentials/role-one":
			if r.Header.Get(tokenHeader) != "token-xyz" {
				t.Fatalf("expected token header for role credentials")
			}
			io.WriteString(w, `{"Code":"Success","AccessKeyId":"AKIA","SecretAccessKey":"secret","Token":"session","Expiration":"2025-10-06T10:00:00Z"}`)
		default:
			http.NotFound(w, r)
		}
	}, func() {
		names, err := GetIAMRoleNames(context.Background())
		require.NoError(t, err)
		require.Equal(t, []string{"role-one"}, names)

		creds, err := GetIAMRoleCredentials(context.Background(), "role-one")
		require.NoError(t, err)
		require.Equal(t, "AKIA", creds.AccessKeyID)
		require.Equal(t, "secret", creds.SecretAccessKey)
		require.Equal(t, "session", creds.SessionToken)
		require.Equal(t, time.Date(2025, 10, 6, 10, 0, 0, 0, time.UTC), creds.Expiration)
	})
}

func TestIMDSRoleProvider(t *testing.T) {
	withMetadataServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest/api/token":
			w.WriteHeader(http.StatusOK)
			io.WriteString(w, "token-abc123")
		case "/latest/meta-data/iam/security-credentials/":
			if r.Header.Get(tokenHeader) != "token-abc123" {
				t.Fatalf("expected token header for role list")
			}
			io.WriteString(w, "role-two\n")
		case "/latest/meta-data/iam/security-credentials/role-two":
			if r.Header.Get(tokenHeader) != "token-abc123" {
				t.Fatalf("expected token header for role credentials")
			}
			io.WriteString(w, `{"Code":"Success","AccessKeyId":"AKIB","SecretAccessKey":"secret2","Token":"session2","Expiration":"2025-10-06T11:00:00Z"}`)
		default:
			http.NotFound(w, r)
		}
	}, func() {
		provider := NewIMDSRoleProvider(context.Background())
		val, err := provider.Retrieve()
		require.NoError(t, err)
		require.Equal(t, "AKIB", val.AccessKeyID)
		require.Equal(t, "secret2", val.SecretAccessKey)
		require.Equal(t, "session2", val.SessionToken)
	})
}

func withMetadataServer(t *testing.T, handler http.HandlerFunc, testFn func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listen: %v", err)
	}
	srv := httptest.NewUnstartedServer(handler)
	srv.Listener = ln
	srv.Start()
	defer srv.Close()

	origTransport := http.DefaultTransport
	host := strings.TrimPrefix(srv.URL, "http://")
	host = strings.TrimPrefix(host, "https://")
	redirect := &metadataRedirectTransport{targetHost: host, base: origTransport}
	http.DefaultTransport = redirect
	defer func() { http.DefaultTransport = origTransport }()

	resetMetadataToken()
	defer resetMetadataToken()

	testFn()
}

type metadataRedirectTransport struct {
	targetHost string
	base       http.RoundTripper
}

func (m *metadataRedirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Host == raddr {
		clone := new(http.Request)
		*clone = *req
		clone.Header = cloneHeader(req.Header)
		u := *req.URL
		u.Host = m.targetHost
		clone.URL = &u
		clone.Host = ""
		return m.base.RoundTrip(clone)
	}
	return m.base.RoundTrip(req)
}

func cloneHeader(h http.Header) http.Header {
	if h == nil {
		return nil
	}
	cp := make(http.Header, len(h))
	for k, v := range h {
		cp[k] = append([]string(nil), v...)
	}
	return cp
}
