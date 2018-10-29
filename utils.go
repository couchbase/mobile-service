package mobile_mds

import (
	"fmt"
	"net/url"
	"strconv"

	"strings"

	"github.com/couchbase/cbauth"
	"github.com/couchbase/gocb"
)

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

// 127.0.0.1:9000 -->  127.0.0.1:9100 (if offset is 100)
func AddPortOffset(hostPort string, offset int) (hostPortWithOffset string, err error) {

	if !strings.Contains(hostPort, ":") {
		return "", fmt.Errorf("Expected : followed by port")
	}

	hostPortComponents := strings.Split(hostPort, ":")
	host := hostPortComponents[0]
	port := hostPortComponents[1]
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return "", err
	}
	portInt += offset

	return fmt.Sprintf("%s:%d", host, portInt), nil

}

func CalculateGrpcPort(hostPort string) (grpcListenPort int, err error) {
	if !strings.Contains(hostPort, ":") {
		return 0, fmt.Errorf("Expected : followed by port")
	}

	hostPortComponents := strings.Split(hostPort, ":")
	port := hostPortComponents[1]
	portInt, err := strconv.Atoi(port)
	if err != nil {
		return 0, err
	}
	return PortGrpcTlsOffset + portInt, nil
}

// http://127.0.0.1:9000 -> 127.0.0.1:9000
func StripHttpScheme(urlWithScheme string) (hostPort string, err error) {
	u, err := url.Parse(urlWithScheme)
	if err != nil {
		return "", err
	}
	hostPort = fmt.Sprintf("%s", u.Host)
	return hostPort, nil
}


func AddTrailingSlash(initial string) string {
	if strings.HasSuffix(initial, "/") {
		return initial
	}
	return fmt.Sprintf("%s/", initial)
}

