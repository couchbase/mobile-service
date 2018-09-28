package mobile_mds

import (
	"net/url"

	"github.com/couchbase/cbauth"
	"github.com/couchbase/gocb"
)

//const (
//	ServerUrl = "http://localhost:8091"
//)

// CBAuthURL rewrites a URL with credentials, for use in a cbauth'ed
// environment.
func CBAuthURL(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	cbUser, cbPasswd, err := cbauth.GetHTTPServiceAuth(u.Host)
	if err != nil {
		return "", err
	}

	u.User = url.UserPassword(cbUser, cbPasswd)

	return u.String(), nil
}

func CBAuthURL2(urlStr string) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return "", err
	}

	cbUser, cbPasswd, err := cbauth.GetMemcachedServiceAuth(u.Host)
	if err != nil {
		return "", err
	}

	u.User = url.UserPassword(cbUser, cbPasswd)

	return u.String(), nil
}

func GetCBAuthMemcachedCreds(urlStr string) (username, password string, err error) {

	u, err := url.Parse(urlStr)
	if err != nil {
		return "", "", err
	}

	return cbauth.GetMemcachedServiceAuth(u.Host)

}

func OpenBucket(bucketName, connSpec string) (bucket *gocb.Bucket, err error) {

	cluster, err := gocb.Connect(connSpec)

	username, password, err := GetCBAuthMemcachedCreds(connSpec)
	if err != nil {
		return nil, err
	}

	authenticator := gocb.PasswordAuthenticator{
		Username: username,
		Password: password,
	}
	cluster.Authenticate(authenticator)

	return cluster.OpenBucket(bucketName, "")

}
