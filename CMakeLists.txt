CMAKE_MINIMUM_REQUIRED (VERSION 2.8)

PROJECT (mobile-service)

INCLUDE (FindCouchbaseGo)

GoInstall (TARGET mobile-service PACKAGE github.com/couchbase/mobile-service/cli
  GOPATH "${PROJECT_SOURCE_DIR}/../../../.." "${GODEPSDIR}"
  GOTAGS ""
  INSTALL_PATH bin OUTPUT mobile-service
  GOVERSION 1.8.5)
