// ABOUTME: Utilities for interacting with AWS EC2 metadata services.
// ABOUTME: Provides helpers for discovering instance identity and devices.
package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rexray/rexray/libstorage/api/types"
	"github.com/rexray/rexray/libstorage/drivers/storage/ebs"
)

const (
	raddr          = "169.254.169.254"
	mdtURL         = "http://" + raddr + "/latest/meta-data/"
	iidURL         = "http://" + raddr + "/latest/dynamic/instance-identity/document"
	bdmURL         = "http://" + raddr + "/latest/meta-data/block-device-mapping/"
	tokenURL       = "http://" + raddr + "/latest/api/token"
	tokenHeader    = "X-aws-ec2-metadata-token"
	tokenTTLHeader = "X-aws-ec2-metadata-token-ttl-seconds"
)

const tokenTTL = 6 * time.Hour

var (
	tokenState struct {
		sync.RWMutex
		token       string
		expiresAt   time.Time
		unsupported bool
	}
)

func resetMetadataToken() {
	tokenState.Lock()
	defer tokenState.Unlock()
	tokenState.token = ""
	tokenState.expiresAt = time.Time{}
	tokenState.unsupported = false
	resetDeviceRange()
}

func maybeAttachIMDSToken(
	ctx types.Context,
	client *http.Client,
	req *http.Request) error {
	if req.URL == nil || req.URL.Host != raddr {
		return nil
	}

	token, err := ensureMetadataToken(ctx, client)
	if err != nil {
		return err
	}
	if token == "" {
		return nil
	}
	if req.Header == nil {
		req.Header = make(http.Header)
	}
	req.Header.Set(tokenHeader, token)
	return nil
}

func ensureMetadataToken(ctx types.Context, client *http.Client) (string, error) {
	tokenState.RLock()
	if !tokenState.unsupported && tokenState.token != "" && time.Now().Before(tokenState.expiresAt) {
		token := tokenState.token
		tokenState.RUnlock()
		return token, nil
	}
	if tokenState.unsupported {
		tokenState.RUnlock()
		return "", nil
	}
	tokenState.RUnlock()

	tokenState.Lock()
	defer tokenState.Unlock()
	if tokenState.unsupported {
		return "", nil
	}
	if tokenState.token != "" && time.Now().Before(tokenState.expiresAt) {
		return tokenState.token, nil
	}

	req, err := http.NewRequest(http.MethodPut, tokenURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set(tokenTTLHeader, strconv.FormatInt(int64(tokenTTL/time.Second), 10))
	req = req.WithContext(ctx)
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK:
		body, err := ioutil.ReadAll(io.LimitReader(res.Body, 1024))
		if err != nil {
			return "", err
		}
		tokenState.token = string(body)
		tokenState.expiresAt = time.Now().Add(tokenTTL)
		return tokenState.token, nil
	case http.StatusNotFound, http.StatusMethodNotAllowed:
		tokenState.unsupported = true
		tokenState.token = ""
		tokenState.expiresAt = time.Time{}
		return "", nil
	case http.StatusForbidden:
		// IMDSv1 hosts may return 403 for unsupported token operations.
		tokenState.unsupported = true
		tokenState.token = ""
		tokenState.expiresAt = time.Time{}
		return "", nil
	}

	body, _ := ioutil.ReadAll(io.LimitReader(res.Body, 1024))
	return "", fmt.Errorf("unexpected metadata token response: status=%d body=%s", res.StatusCode, string(body))
}

// IsEC2Instance returns a flag indicating whether the executing host is an EC2
// instance based on whether or not the metadata URL can be accessed.
func IsEC2Instance(ctx types.Context) (bool, error) {
	client := &http.Client{Timeout: time.Duration(1 * time.Second)}
	req, err := http.NewRequest(http.MethodHead, mdtURL, nil)
	if err != nil {
		return false, err
	}
	res, err := doRequestWithClient(ctx, client, req)
	if err != nil {
		if terr, ok := err.(net.Error); ok && terr.Timeout() {
			return false, nil
		}
		return false, err
	}
	if res.StatusCode >= 200 || res.StatusCode <= 299 {
		return true, nil
	}
	return false, nil
}

type instanceIdentityDoc struct {
	InstanceID       string `json:"instanceId,omitempty"`
	Region           string `json:"region,omitempty"`
	AvailabilityZone string `json:"availabilityZone,omitempty"`
}

// InstanceID returns the instance ID for the local host.
func InstanceID(
	ctx types.Context,
	driverName string) (*types.InstanceID, error) {

	req, err := http.NewRequest(http.MethodGet, iidURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	iid := instanceIdentityDoc{}
	dec := json.NewDecoder(res.Body)
	if err := dec.Decode(&iid); err != nil {
		return nil, err
	}

	return &types.InstanceID{
		ID:     iid.InstanceID,
		Driver: driverName,
		Fields: map[string]string{
			ebs.InstanceIDFieldRegion:           iid.Region,
			ebs.InstanceIDFieldAvailabilityZone: iid.AvailabilityZone,
		},
	}, nil
}

// BlockDevices returns the EBS devices attached to the local host.
func BlockDevices(ctx types.Context) ([]byte, error) {

	req, err := http.NewRequest(http.MethodGet, bdmURL, nil)
	if err != nil {
		return nil, err
	}

	res, err := doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

// BlockDeviceName returns the name of the provided EBS device.
func BlockDeviceName(
	ctx types.Context,
	device string) ([]byte, error) {

	req, err := http.NewRequest(
		http.MethodGet,
		fmt.Sprintf("%s%s", bdmURL, device),
		nil)
	if err != nil {
		return nil, err
	}

	res, err := doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	buf, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return buf, nil
}
