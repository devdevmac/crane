package registry

import (
	"crypto"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Dataman-Cloud/rolex/util/config"

	"github.com/docker/distribution/registry/auth/token"
	"github.com/docker/libtrust"
)

const (
	issuer     = "dataman-inc"
	expiration = 5 //minute
)

// GetResourceActions ...
func ParseResourceActions(scope string) []*token.ResourceActions {
	var res []*token.ResourceActions
	if scope == "" {
		return res
	}
	items := strings.Split(scope, ":")
	res = append(res, &token.ResourceActions{
		Type:    items[0],
		Name:    items[1],
		Actions: strings.Split(items[2], ","),
	})
	return res
}

// FilterAccess modify the action list in access based on permission
// determine if the request needs to be authenticated.
func FilterAccess(username string, authenticated bool, a *token.ResourceActions) {

	if a.Type == "registry" && a.Name == "catalog" {
		return
	}

	//clear action list to assign to new acess element after perm check.
	a.Actions = []string{}
	if a.Type == "repository" {
		if strings.Contains(a.Name, "/") { //Only check the permission when the requested image has a namespace, i.e. project
			// TODO check if account has read/write permission
			projectName := a.Name[0:strings.LastIndex(a.Name, "/")]
			var permission string
			if authenticated {
				if username == "admin" {
					permission = "RW"
				} else {
					permission = GetPermission(username, projectName)
				}
			}
			if strings.Contains(permission, "W") {
				a.Actions = append(a.Actions, "push")
			}
			if strings.Contains(permission, "R") {
				a.Actions = append(a.Actions, "pull")
			}
		}
	}
	fmt.Printf("current access, type: %s, name:%s, actions:%v \n", a.Type, a.Name, a.Actions)
}

func MakeToken(config *config.Config, username, service string, access []*token.ResourceActions) (string, error) {
	pk, err := libtrust.LoadKeyFile(config.RegistryPrivateKeyPath)
	if err != nil {
		return "", err
	}
	tk, err := makeTokenCore(issuer, username, service, expiration, access, pk)
	if err != nil {
		return "", err
	}
	rs := fmt.Sprintf("%s.%s", tk.Raw, base64UrlEncode(tk.Signature))
	return rs, nil
}

//make token core
func makeTokenCore(issuer, subject, audience string, expiration int,
	access []*token.ResourceActions, signingKey libtrust.PrivateKey) (*token.Token, error) {

	joseHeader := &token.Header{
		Type:       "JWT",
		SigningAlg: "RS256",
		KeyID:      signingKey.KeyID(),
	}

	jwtID, err := randString(16)
	if err != nil {
		return nil, fmt.Errorf("Error to generate jwt id: %s", err)
	}

	now := time.Now()

	claimSet := &token.ClaimSet{
		Issuer:     issuer,
		Subject:    subject,
		Audience:   audience,
		Expiration: now.Add(time.Duration(expiration) * time.Minute).Unix(),
		// for testing purpose
		//IssuedAt:   now.Add(-1 * time.Duration(644) * time.Second).Unix(),
		//NotBefore:  now.Add(-1 * time.Duration(644) * time.Second).Unix(),
		NotBefore: now.Unix(),
		IssuedAt:  now.Unix(),
		JWTID:     jwtID,
		Access:    access,
	}

	var joseHeaderBytes, claimSetBytes []byte

	if joseHeaderBytes, err = json.Marshal(joseHeader); err != nil {
		return nil, fmt.Errorf("unable to marshal jose header: %s", err)
	}
	if claimSetBytes, err = json.Marshal(claimSet); err != nil {
		return nil, fmt.Errorf("unable to marshal claim set: %s", err)
	}

	encodedJoseHeader := base64UrlEncode(joseHeaderBytes)
	encodedClaimSet := base64UrlEncode(claimSetBytes)
	payload := fmt.Sprintf("%s.%s", encodedJoseHeader, encodedClaimSet)

	var signatureBytes []byte
	if signatureBytes, _, err = signingKey.Sign(strings.NewReader(payload), crypto.SHA256); err != nil {
		return nil, fmt.Errorf("unable to sign jwt payload: %s", err)
	}

	signature := base64UrlEncode(signatureBytes)
	tokenString := fmt.Sprintf("%s.%s", payload, signature)
	return token.NewToken(tokenString)
}

func randString(length int) (string, error) {
	const alphanum = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	rb := make([]byte, length)
	_, err := rand.Read(rb)
	if err != nil {
		return "", err
	}
	for i, b := range rb {
		rb[i] = alphanum[int(b)%len(alphanum)]
	}
	return string(rb), nil
}

func base64UrlEncode(b []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(b), "=")
}

func GetPermission(username, project string) string {
	return "RW"
}
