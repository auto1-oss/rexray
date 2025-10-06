package utils

// ABOUTME: Helpers for retrieving IAM role credentials via EC2 IMDSv2.
// ABOUTME: Supplies a credentials.Provider compatible with AWS SDK v1.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws/credentials"

	"github.com/rexray/rexray/libstorage/api/types"
)

const (
	iamSecurityCredsPath = "iam/security-credentials/"
	imdsRoleProviderName = "IMDSv2RoleProvider"
)

// IAMRoleCredentials represents temporary credentials retrieved from IMDS.
type IAMRoleCredentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Expiration      time.Time
}

type iamRoleCredsResponse struct {
	Code            string    `json:"Code"`
	AccessKeyID     string    `json:"AccessKeyId"`
	SecretAccessKey string    `json:"SecretAccessKey"`
	Token           string    `json:"Token"`
	Expiration      time.Time `json:"Expiration"`
	Message         string    `json:"Message"`
}

// GetIAMRoleNames returns the role names available via IMDS.
func GetIAMRoleNames(ctx types.Context) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, mdtURL+iamSecurityCredsPath, nil)
	if err != nil {
		return nil, err
	}

	res, err := doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	roles := []string{}
	scanner := bufio.NewScanner(strings.NewReader(string(body)))
	for scanner.Scan() {
		role := strings.TrimSpace(scanner.Text())
		if role != "" {
			roles = append(roles, role)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return roles, nil
}

// GetIAMRoleCredentials fetches role credentials for the given role name.
func GetIAMRoleCredentials(ctx types.Context, role string) (*IAMRoleCredentials, error) {
	rolePath := iamSecurityCredsPath + role
	req, err := http.NewRequest(
		http.MethodGet,
		mdtURL+rolePath,
		nil,
	)
	if err != nil {
		return nil, err
	}

	res, err := doRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	decoder := json.NewDecoder(res.Body)
	var resp iamRoleCredsResponse
	if err := decoder.Decode(&resp); err != nil {
		return nil, err
	}

	if resp.Code != "Success" {
		return nil, fmt.Errorf("imds role credentials error: %s", resp.Message)
	}

	return &IAMRoleCredentials{
		AccessKeyID:     resp.AccessKeyID,
		SecretAccessKey: resp.SecretAccessKey,
		SessionToken:    resp.Token,
		Expiration:      resp.Expiration,
	}, nil
}

type imdsRoleProvider struct {
	credentials.Expiry
	ctx types.Context
}

// NewIMDSRoleProvider returns a credentials.Provider backed by IMDSv2.
func NewIMDSRoleProvider(ctx types.Context) credentials.Provider {
	return &imdsRoleProvider{ctx: ctx}
}

func (p *imdsRoleProvider) Retrieve() (credentials.Value, error) {
	roles, err := GetIAMRoleNames(p.ctx)
	if err != nil {
		return credentials.Value{ProviderName: imdsRoleProviderName}, err
	}
	if len(roles) == 0 {
		return credentials.Value{ProviderName: imdsRoleProviderName}, fmt.Errorf("no iam role names found")
	}

	creds, err := GetIAMRoleCredentials(p.ctx, roles[0])
	if err != nil {
		return credentials.Value{ProviderName: imdsRoleProviderName}, err
	}

	p.SetExpiration(creds.Expiration, 5*time.Minute)

	return credentials.Value{
		AccessKeyID:     creds.AccessKeyID,
		SecretAccessKey: creds.SecretAccessKey,
		SessionToken:    creds.SessionToken,
		ProviderName:    imdsRoleProviderName,
	}, nil
}

func (p *imdsRoleProvider) IsExpired() bool {
	return p.Expiry.IsExpired()
}
