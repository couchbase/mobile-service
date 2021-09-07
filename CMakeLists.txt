CMAKE_MINIMUM_REQUIRED (VERSION 3.13)

PROJECT (mobile-service)

INCLUDE (FindCouchbaseGo)

GoInstall (TARGET mobile-service PACKAGE github.com/couchbase/mobile-service/cli
  GOPATH "${PROJECT_SOURCE_DIR}/../../../.." "${GODEPSDIR}"
  GOTAGS ""
  INSTALL_PATH bin OUTPUT mobile-service
  GOVERSION 1.11.6)
