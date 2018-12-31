package mobile_service

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/couchbase/cbauth/metakv"
	"github.com/couchbase/cbauth/service"
	"github.com/couchbase/mobile-service/mobile_service_grpc"
	"path"
)

// Implements the MobileServiceAdmin grpc service defined in mobile_service_admin.proto
type mobileServiceAdmin struct {

	// The node UUID for this mobile service node.  Provided by ns-server during startup
	NodeUuid service.NodeID
}

func NewMobileServiceAdmin(nodeUuid service.NodeID) *mobileServiceAdmin {

	s := mobileServiceAdmin{
		NodeUuid: nodeUuid,
	}

	return &s
}

func (self *mobileServiceAdmin) ListSyncGateways(context.Context, *mobile_service_grpc.Empty) (*mobile_service_grpc.SyncGateways, error) {

	uniqueGateways := map[string]*mobile_service_grpc.SyncGateway{}

	knownSyncGateways, err := metakv.ListAllChildren(AddTrailingSlash(KeyMobileState))
	if err != nil {
		return nil, err
	}

	for _, knownSyncGateway := range knownSyncGateways {

		syncGateway := &mobile_service_grpc.SyncGateway{}
		syncGateway.SyncGatewayUUID = path.Base(knownSyncGateway.Path)
		syncGateway.LastSeenTimestamp = string(knownSyncGateway.Value)
		uniqueGateways[syncGateway.SyncGatewayUUID] = syncGateway

	}

	gatewayList := []*mobile_service_grpc.SyncGateway{}
	for _, uniqueGateway := range uniqueGateways {
		gatewayList = append(gatewayList, uniqueGateway)
	}
	gateways := &mobile_service_grpc.SyncGateways{}
	gateways.Items = gatewayList

	return gateways, nil

}

func (self *mobileServiceAdmin) ImportConfig(ctx context.Context, config *mobile_service_grpc.ConfigJson) (empty *mobile_service_grpc.Empty, err error) {

	configImportExport := ConfigImportExport{}

	if err := json.Unmarshal([]byte(config.Body), &configImportExport); err != nil {
		return &mobile_service_grpc.Empty{}, err
	}

	// Set general config
	if err := metakv.Set(KeyMobileGatewayGeneral, configImportExport.General, nil); err != nil {
		return &mobile_service_grpc.Empty{}, err
	}

	// Set listener config
	if err := metakv.Set(KeyMobileGatewayListener, configImportExport.Listener, nil); err != nil {
		return &mobile_service_grpc.Empty{}, err
	}

	// Set databases config
	for dbName, dbVal := range configImportExport.Databases {
		metakvPath := fmt.Sprintf("%s/%s", KeyMobileGatewayDatabases, dbName)
		if err := metakv.Set(metakvPath, dbVal, nil); err != nil {
			return &mobile_service_grpc.Empty{}, err
		}
	}

	return &mobile_service_grpc.Empty{}, nil

}

func (self *mobileServiceAdmin) ExportConfig(ctx context.Context, empty *mobile_service_grpc.Empty) (config *mobile_service_grpc.ConfigJson, err error) {

	config = &mobile_service_grpc.ConfigJson{}

	configImportExport := ConfigImportExport{}
	configImportExport.Databases = map[string]json.RawMessage{}

	var val []byte

	val, _, err = metakv.Get(KeyMobileGatewayGeneral)
	if err != nil {
		return config, err
	}
	configImportExport.General = val

	val, _, err = metakv.Get(KeyMobileGatewayListener)
	if err != nil {
		return config, err
	}
	configImportExport.Listener = val

	// List all the chidren of the metakvDatabases key
	kvEntriesDbs, err := metakv.ListAllChildren(AddTrailingSlash(KeyMobileGatewayDatabases))
	if err != nil {
		return config, err
	}

	// Loop over entries and add to configImportExport
	for _, dbKvEntry := range kvEntriesDbs {
		configImportExport.Databases[path.Base(dbKvEntry.Path)] = dbKvEntry.Value
	}

	marshalled, err := json.Marshal(configImportExport)
	if err != nil {
		return config, err
	}

	config.Body = string(marshalled)

	return config, nil
}
