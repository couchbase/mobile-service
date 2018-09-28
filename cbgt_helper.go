package mobile_mds

import (
	"strings"

	"fmt"
	"os"

	"encoding/json"

	"log"

	"net/http"
	"time"

	"net"

	"github.com/couchbase/cbauth/service"
	"github.com/couchbase/cbgt"
	"github.com/couchbase/cbgt/cmd"
	"github.com/couchbase/cbgt/ctl"
	cbgtrest "github.com/couchbase/cbgt/rest"
	"github.com/gorilla/mux"
)

const (
	IndexCategoryMobileService = "general"
	MobileServiceBaseName      = "MobileService"
	SourceTypeCouchbase        = "couchbase"
	CbgtRestApiPort            = 4988 // TODO: figure out if we even need to bind to a port, since colocated with CBFT
	NumCbgtShards              = 8
	TestIndexName              = "test_index"
	TestBucketName             = "default"
	CbgtMobileCfgMetaKvPrefix  = "/mobile/cbgt/cfg/" // Customize this to avoid conflicts with CBFT
)

type MobileServiceIndexType int

const (
	ConvergenceImport MobileServiceIndexType = iota

	SgReplicate
)

func init() {
	cbgt.CfgMetaKvPrefix = CbgtMobileCfgMetaKvPrefix
}

func (m MobileServiceIndexType) String() string {
	switch m {
	case ConvergenceImport:
		return "ConvergenceImport"
	case SgReplicate:
		return "SgReplicate"
	default:
		return "Error"
	}
}

type CbgtHelper struct {
	cfg         cbgt.Cfg
	manager     *cbgt.Manager
	ctlInstance *ctl.Ctl
	ctlMgr      *ctl.CtlMgr
	Params      CbgtParams
}

func InitCBGT(params CbgtParams) (*CbgtHelper, error) {

	log.Printf("InitCBGT called with params: %+v", params)

	cbgtHelper, err := NewCbgtHelper(params)
	if err != nil {
		return nil, err
	}

	manager, err := cbgtHelper.CreateManager()
	if err != nil {
		return nil, err
	}

	// this tells CBGT that we are brining a new CBGT node online
	register := "wanted"
	err = manager.Start(register)
	if err != nil {
		return nil, err
	}

	cbgtHelper.RegisterIndexTypes()

	if params.CreateIndex {
		indexParams := CbgtIndexParams{
			BucketName: TestBucketName,
			IndexType:  ConvergenceImport,
			IndexName:  TestIndexName,
			NumShards:  NumCbgtShards,
		}
		if err := cbgtHelper.CreateIndex(indexParams); err != nil {
			return nil, err
		}

		cbgtHelper.InspectIndex(indexParams)
	}

	go func() {
		if err := cbgtHelper.StartRESTServer(); err != nil {
			panic(fmt.Sprintf("Error running REST server: %v", err))
		}
	}()

	ctlInstance, err := cbgtHelper.StartCtlInstance()
	if err != nil {
		return nil, err
	}

	log.Printf("CtlInstance: %v", ctlInstance)

	if err := cbgtHelper.StartCtlManager(); err != nil {
		return nil, err
	}

	return cbgtHelper, nil
}

func NewCbgtHelper(params CbgtParams) (*CbgtHelper, error) {

	if params.NodeUuid == "" {
		log.Printf("Creaing a CBGT UUID in dataDir: %v", params.DataDir)
		var err error
		params.NodeUuid, err = cmd.MainUUID(MobileServiceBaseName, params.DataDir)
		if err != nil {
			return nil, err
		}
	} else {

		// TODO: if a node uuid is passed in, which is the default behavior when
		// TODO: invoked by ns-server, this should write the node uuid to the same file that would be written
		// TODO: above, in order to match CBFT behavior.

	}

	return &CbgtHelper{
		Params: params,
	}, nil

}

func (h *CbgtHelper) CreateCfg() error {
	cfgMetaKv, err := cbgt.NewCfgMetaKv(h.Params.NodeUuid)
	if err != nil {
		return err
	}
	h.cfg = cfgMetaKv
	return nil
}

func (h *CbgtHelper) CreateManager() (*cbgt.Manager, error) {

	// Create a CBGT Cfg if we don't already have one
	if h.cfg == nil {
		if err := h.CreateCfg(); err != nil {
			return nil, err
		}
	}

	// use the UUID as the bindHttp so that we don't have to make the user
	// configure this, and since as far as the REST Api interface, we'll be using
	// whatever is configured in adminInterface anyway.
	// More info here:
	//   https://github.com/couchbaselabs/cbgt/issues/1
	//   https://github.com/couchbaselabs/cbgt/issues/25
	// bindHttp := h.UUID

	// In order to be able to access the CBGT REST API, I'm going to try to expose it on a port.
	// I believe bindHttp should match up with that port, not 100% sure if this is needed though.
	// The actual setup of the http listener happens outside of CBGT.
	bindHttp := fmt.Sprintf(":%d", CbgtRestApiPort)

	tags := []string{"feed", "janitor", "pindex", "planner", "cbauth_service"}
	container := ""
	weight := 1 // this would allow us to have more pindexes serviced by this node
	extras := ""
	var managerEventHandlers cbgt.ManagerEventHandlers

	manager := cbgt.NewManager(
		cbgt.VERSION,
		h.cfg,
		h.Params.NodeUuid,
		tags,
		container,
		weight,
		extras,
		bindHttp,
		h.Params.DataDir,
		strings.Join(h.Params.Urls, ";"),
		managerEventHandlers,
	)

	h.manager = manager

	return h.manager, nil

}

// Start a rest server -- this blocks forever and so
// you should run this in a goroutine if you don't want to block.
func (h *CbgtHelper) StartRESTServer() error {

	router, err := h.CreateRESTRouter()
	if err != nil {
		return err
	}

	h.Params.MaybeAddArtificalDelay()

	// Keep trying to bind to ports, starting with CbgtRestApiPort
	// TODO: the CbgtRestApiPort should be passed in as a CLI
	for i := 0; i < 100; i++ {
		portNumber := CbgtRestApiPort + i
		bindHttp := fmt.Sprintf(":%d", portNumber)

		server := &http.Server{
			Addr:         bindHttp,
			Handler:      router,
			ReadTimeout:  time.Second * 60,
			WriteTimeout: time.Second * 60,
		}

		log.Printf("Attempting to bind to %d", portNumber)
		listener, err := net.Listen("tcp", bindHttp)
		if err != nil {
			log.Printf("Error binding to %d: %v.  Will retry.", portNumber, err)
			continue
		}
		return server.Serve(listener)
	}

	return fmt.Errorf("Unable to bind to any ports after several tries")

}

func (h *CbgtHelper) CreateRESTRouter() (*mux.Router, error) {

	messageRing, err := cbgt.NewMsgRing(os.Stderr, 1000)
	if err != nil {
		return nil, err
	}

	router, _, err := cbgtrest.NewRESTRouter(
		"v0",
		h.manager,
		"static",
		"",
		messageRing,
		nil,
		nil)

	if err != nil {
		return nil, err
	}

	return router, nil

}

func (h *CbgtHelper) getPreviousCBGTIndexUUID(indexParams CbgtIndexParams) (previousUUID string, err error) {

	_, indexDefsMap, err := h.manager.GetIndexDefs(true)
	if err != nil {
		return "", err
	}

	indexDef, ok := indexDefsMap[indexParams.IndexName]
	if ok {
		return indexDef.UUID, nil
	} else {
		return "", nil
	}

}

func (h *CbgtHelper) InspectIndex(indexParams CbgtIndexParams) {
	planPindexes, cas, err := cbgt.CfgGetPlanPIndexes(h.cfg)
	log.Printf("CfgGetPlanPIndexes: %v, cas: %v, err: %v", planPindexes, cas, err)

}

func (h *CbgtHelper) CreateIndex(indexParams CbgtIndexParams) error {

	username, password, err := GetCBAuthMemcachedCreds(h.Params.Urls[0])
	if err != nil {
		return err
	}

	sourceParams := cbgt.NewDCPFeedParams()
	// leave user and pwd blank, hopefully cbauth will kick in
	sourceParams.AuthUser = username
	sourceParams.AuthPassword = password
	sourceParams.IncludeXAttrs = true

	log.Printf("Connecting to DCP feed with params: %+v", sourceParams)

	sourceParamsBytes, err := json.Marshal(sourceParams)
	if err != nil {
		return err
	}

	bucket, err := OpenBucket(indexParams.BucketName, h.Params.Urls[0])
	if err != nil {
		return err
	}

	indexParamsBytes, err := json.Marshal(indexParams)
	if err != nil {
		return err
	}

	// If the index has already been created, get the previous UUID to use during upsert
	previousUUID, err := h.getPreviousCBGTIndexUUID(indexParams)
	if err != nil {
		return err
	}

	numVbucketsTotal := uint16(bucket.IoRouter().NumVbuckets())

	log.Printf("Creating cbgt index against bucket: %s/%s with params: %+v",
		bucket.Name(), bucket.IoRouter().BucketUUID(), indexParams)

	err = h.manager.CreateIndex(
		SourceTypeCouchbase,                         // sourceType
		bucket.Name(),                               // sourceName
		bucket.IoRouter().BucketUUID(),              // sourceUUID
		string(sourceParamsBytes),                   // sourceParams
		indexParams.IndexType.String(),              // IndexType
		indexParams.IndexName,                       // IndexName
		string(indexParamsBytes),                    // indexParams
		indexParams.GetPlanParams(numVbucketsTotal), // planParams
		previousUUID,                                // prevIndexUUID
	)
	if err != nil {
		return err
	}

	return nil

}

func (h *CbgtHelper) RegisterIndexTypes() {

	for _, indexType := range h.Params.IndexTypes {

		// The Description format is:
		//
		//     $categoryName/$IndexType - short descriptive string
		//
		// where $categoryName is something like "advanced", or "general".
		description := fmt.Sprintf("%v/%v - Mobile Service", IndexCategoryMobileService, indexType)

		switch indexType {
		case ConvergenceImport:
			cbgt.RegisterPIndexImplType(indexType.String(), &cbgt.PIndexImplType{
				New:         h.NewConvergenceImportPIndexFactory,
				Open:        h.OpenConvergenceImportPIndexFactory,
				Count:       nil,
				Query:       nil,
				Description: description,
			})
		case SgReplicate:
			cbgt.RegisterPIndexImplType(indexType.String(), &cbgt.PIndexImplType{
				New:         h.NewSgReplicatePIndexFactory,
				Open:        h.OpenSgReplicatePIndexFactory,
				Count:       nil,
				Query:       nil,
				Description: description,
			})
		}

	}

}

// When CBGT is opening a PIndex for the first time, it will call back this method.
func (h *CbgtHelper) NewConvergenceImportPIndexFactory(indexType, indexParams, path string, restart func()) (cbgt.PIndexImpl, cbgt.Dest, error) {

	os.MkdirAll(path, 0700)

	result := &ConvergenceImportPIndex{}

	return result, result, nil

}

// When CBGT is re-opening an existing PIndex (after a Sync Gw restart for example), it will call back this method.
func (h *CbgtHelper) OpenConvergenceImportPIndexFactory(indexType, path string, restart func()) (cbgt.PIndexImpl, cbgt.Dest, error) {

	os.MkdirAll(path, 0700)

	result := &ConvergenceImportPIndex{}

	return result, result, nil

}

// When CBGT is opening a PIndex for the first time, it will call back this method.
func (h *CbgtHelper) NewSgReplicatePIndexFactory(indexType, indexParams, path string, restart func()) (cbgt.PIndexImpl, cbgt.Dest, error) {

	os.MkdirAll(path, 0700)

	result := &SgReplicatePIndex{}

	return result, result, nil

}

// When CBGT is re-opening an existing PIndex (after a Sync Gw restart for example), it will call back this method.
func (h *CbgtHelper) OpenSgReplicatePIndexFactory(indexType, path string, restart func()) (cbgt.PIndexImpl, cbgt.Dest, error) {

	os.MkdirAll(path, 0700)

	result := &SgReplicatePIndex{}

	return result, result, nil

}

func (h *CbgtHelper) StartCtlInstance() (*ctl.Ctl, error) {

	maxConcurrentPartitionMovesPerNode := 1

	dryRun := false
	verbose := 3
	waitForMemberNodes := 30 // In seconds.

	var ctlInstance *ctl.Ctl
	ctlInstance, err := ctl.StartCtl(h.cfg, h.Params.Urls[0], h.manager.Options(), ctl.CtlOptions{
		DryRun:                             dryRun,
		Verbose:                            verbose,
		FavorMinNodes:                      false,
		WaitForMemberNodes:                 waitForMemberNodes,
		MaxConcurrentPartitionMovesPerNode: maxConcurrentPartitionMovesPerNode,
		Manager:       h.manager,
		SkipSeqChecks: true, // Workaround the rebalancer errors: "rebalance: waitAssignPIndexDone, awaiting a stats sample grab for pindex test_index_30d002451d85c194_e3912416"
	})

	if err == nil {
		h.ctlInstance = ctlInstance
	}
	return ctlInstance, err

}

func (h *CbgtHelper) StartCtlManager() error {

	err := h.cfg.Refresh()
	if err != nil {
		return err
	}

	nodeInfo := &service.NodeInfo{
		NodeID: service.NodeID(h.Params.NodeUuid),
	}

	ctlMgr := ctl.NewCtlMgr(nodeInfo, h.ctlInstance)
	log.Printf("ctlMgr created %v with instance: %v", ctlMgr, h.ctlInstance)

	if ctlMgr != nil {
		h.ctlMgr = ctlMgr
		go func() {
			log.Printf("calling service.RegisterManager with %v", ctlMgr)
			err = service.RegisterManager(ctlMgr, nil)
			log.Printf("called service.RegisterManager with %v, got: %v", ctlMgr, err)

			if err != nil {
				panic(fmt.Sprintf("Error calling cbauth service.RegisterManager: %v", err))
			}
			log.Printf("ctlMgr registered to cbauth service: %v", ctlMgr)
		}()
	}

	return nil

}
