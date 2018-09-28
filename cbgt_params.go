package mobile_mds

import (
	"strings"
	"time"
)

// Parameters needed by the Mobile Service node
type MobileServiceNodeParams struct {

	// The node UUID, or empty if none passed in
	NodeUuid string

	// The path where this MobileServiceNode should store any local data
	DataDir string

	// The list of URLS to connect to Couchbase Server (port 8091)
	Urls []string
}

type CbgtParams struct {

	MobileServiceNodeParams

	// By default, don't create an Index because there might not even be a bucket defined.
	// But if you have manually created a "default" bucket, set this to true to create an index.
	CreateIndex bool

	IndexTypes []MobileServiceIndexType
}



// Artificial delay hack to try to ensure that it's always the
// following mapping when doing "cluster_run -n 2":
//
// CBS Server port 9000 -> Mobile MDS port CbgtRestApiPort
// CBS Server port 9001 -> Mobile MDS port CbgtRestApiPort + 1
func (p CbgtParams) MaybeAddArtificalDelay() {

	if strings.Contains(p.Urls[0], "9001") {
		time.Sleep(time.Second * 1)
	}
}
