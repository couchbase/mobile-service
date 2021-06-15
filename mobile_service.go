package mobile_service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/couchbase/cbauth"
	"github.com/couchbase/cbauth/metakv"
	"github.com/couchbase/cbauth/service"
	msgrpc "github.com/couchbase/mobile-service/mobile_service_grpc"
)

// Implements the MobileService grpc service defined in mobile_service.proto
type mobileService struct {

	// The node UUID for this mobile service node.  Provided by ns-server during startup
	NodeUuid service.NodeID

	// This can probably be deleted since it's already being stored in MetaKV
	LastSeenGateways *LastSeenMap
}

func NewMobileService(nodeUuid service.NodeID) *mobileService {

	s := mobileService{
		NodeUuid:         nodeUuid,
		LastSeenGateways: NewLastSeenMap(),
	}

	go s.DeleteStaleGatewayEntries()

	return &s
}

func (s *mobileService) DeleteStaleGatewayEntries() {

	for {

		log.Printf("Checking for stale entries in s.LastSeenGateways.StaleEntries")

		staleEntries := s.LastSeenGateways.StaleEntries(time.Second * 10)

		log.Printf("staleEntries: %v", staleEntries)

		for _, staleEntryKeyPath := range staleEntries {

			log.Printf("Deleting gateway from /mobile/state since has not been seen lately: %v.  First do Get()", staleEntryKeyPath)

			_, rev, _ := metakv.Get(staleEntryKeyPath)

			log.Printf("Delete(): %v", staleEntryKeyPath)

			metakv.Delete(staleEntryKeyPath, rev)

		}

		s.LastSeenGateways.DeleteEntries(staleEntries)

		time.Sleep(time.Second * 10)
	}

}

func (s *mobileService) SendStats(stream msgrpc.MobileService_SendStatsServer) error {
	for {
		stats, streamRecvErr := stream.Recv()

		log.Printf("GrpcServer SendStats() stream received stats: %v.", stats)

		if streamRecvErr != nil {
			// TODO: There should be a pre-existing association of the stream with the nodeuuid
			// TODO: and then at this point it could lookup the node id based on the stream, and immediately
			// TODO: remove presence status from MetaKV.  (it will still get cleaned up w/o this, but this
			// TODO: would decrease latency)
			log.Printf("GrpcServer SendStats() received err: %v.", streamRecvErr)
			return streamRecvErr
		}

		// Authentication
		creds, cbAuthErr := cbauth.Auth(stats.Creds.Username, stats.Creds.Password)
		if cbAuthErr != nil {
			log.Printf("GrpcServer auth err: %v.", cbAuthErr)
			continue
		}

		// Check permissions
		allowed, credsErr := creds.IsAllowed("cluster.settings!write")
		if credsErr != nil {
			log.Printf("GrpcServer auth err checking permissions: %v.", credsErr)
			continue
		}
		if !allowed {
			log.Printf("GrpcServer permission denied")
			continue
		}

		// Record heartbeat
		heartBeatErr := s.RecordClientHeartbeat(stats)

		// Handle errors
		if heartBeatErr != nil {
			if heartBeatErr == io.EOF {
				return stream.SendAndClose(&msgrpc.StatsReply{})
			}
			return heartBeatErr
		}

	}

}

func (s *mobileService) RecordClientHeartbeat(stats *msgrpc.Stats) error {

	keypath := fmt.Sprintf("%s/%s/%s", KeyMobileState, s.NodeUuid, stats.Gateway.Uuid)

	// Update entry in LastSeenGateways
	s.LastSeenGateways.UpdateLastSeen(keypath)

	// Add/update entry in metakv

	// Add a value associated w/ this gateway node.  For now just put the current timestamp
	gatewayState := []byte(fmt.Sprintf("%s", time.Now()))

	log.Printf("Recording client heartbeat to metakv: %v", keypath)

	// Try to add
	if err := metakv.Add(keypath, gatewayState); err != nil {

		// Key exist, get latest rev and try set
		_, rev, _ := metakv.Get(keypath)

		if err := metakv.Set(keypath, gatewayState, rev); err != nil {
			return err
		}
	}

	return nil

}

func (s *mobileService) MetaKVGet(context context.Context, metaKVPath *msgrpc.MetaKVPath) (*msgrpc.MetaKVPair, error) {

	val, rev, err := metakv.Get(metaKVPath.Path)
	if err != nil {
		return &msgrpc.MetaKVPair{}, err
	}

	revStr, err := RevString(rev)
	if err != nil {
		return &msgrpc.MetaKVPair{}, err
	}
	log.Printf("Rev marshalled: %s", string(revStr))

	return &msgrpc.MetaKVPair{
		Value: val,
		Rev:   revStr,
		Path:  metaKVPath.Path,
	}, nil

}

// Convert a rev interface{} -> string in order to pass to GRPC
// TODO: clean up mess .. just doing rapid prototyping
func RevString(rev interface{}) (string, error) {
	if rev == nil {
		return "", nil
	}
	revBytes, err := json.Marshal(rev)
	if err != nil {
		return "", err
	}
	if len(revBytes) == 0 {
		return "", nil
	}
	if !strings.Contains(string(revBytes), `""`) {
		return string(revBytes), nil
	}
	return strconv.Unquote(string(revBytes))
}

func (s *mobileService) MetaKVSet(context context.Context, metaKVPair *msgrpc.MetaKVPair) (*msgrpc.Empty, error) {

	// TODO: verify that they are under the /mobile key space

	log.Printf("Updating key pair: %+v", metaKVPair)

	// TODO: pass in rev from the metaKVPair.  Running into errors when trying to go from interface{} -> string -> interface{}
	if err := metakv.Set(metaKVPair.Path, []byte(metaKVPair.Value), nil); err != nil {

		log.Printf("Error updating key pair: %+v.  Err: %v", metaKVPair, err)

		return &msgrpc.Empty{}, err
	}

	log.Printf("Updated key pair: %+v", metaKVPair)

	return &msgrpc.Empty{}, nil

}

func (s *mobileService) MetaKVAdd(context context.Context, metaKVPair *msgrpc.MetaKVPair) (*msgrpc.Empty, error) {

	if err := metakv.Add(metaKVPair.Path, []byte(metaKVPair.Value)); err != nil {
		return &msgrpc.Empty{}, err
	}

	return &msgrpc.Empty{}, nil

}

func (s *mobileService) MetaKVDelete(context context.Context, metaKVPair *msgrpc.MetaKVPair) (*msgrpc.Empty, error) {

	log.Printf("Deleting key pair: %+v", metaKVPair)

	// TODO: pass in rev from the metaKVPair.  Running into errors when trying to go from interface{} -> string -> interface{}
	if err := metakv.Delete(metaKVPair.Path, nil); err != nil {
		log.Printf("Error deleting key pair: %+v.  Err: %v", metaKVPair, err)
		return &msgrpc.Empty{}, err
	}

	log.Printf("Deleted key pair: %+v", metaKVPair)

	return &msgrpc.Empty{}, nil

}

func (s *mobileService) MetaKVRecursiveDelete(context context.Context, metaKVPath *msgrpc.MetaKVPath) (*msgrpc.Empty, error) {
	if err := metakv.RecursiveDelete(metaKVPath.Path); err != nil {
		log.Printf("Error deleting key path: %+v.  Err: %v", metaKVPath, err)
		return &msgrpc.Empty{}, err
	}
	return &msgrpc.Empty{}, nil
}

func (s *mobileService) MetaKVListAllChildren(context context.Context, metaKVPath *msgrpc.MetaKVPath) (*msgrpc.MetaKVPairs, error) {

	entries, err := metakv.ListAllChildren(metaKVPath.Path)
	if err != nil {
		return &msgrpc.MetaKVPairs{}, err
	}

	items := make([]*msgrpc.MetaKVPair, len(entries))
	for i, entry := range entries {

		revStr, err := RevString(entry.Rev)
		if err != nil {
			log.Printf("Error converting rev to string: %v", err)
		}

		items[i] = &msgrpc.MetaKVPair{
			Path:  entry.Path,
			Rev:   revStr,
			Value: entry.Value,
		}
	}

	metakvPairs := msgrpc.MetaKVPairs{
		Items: items,
	}

	return &metakvPairs, nil

}

func (s *mobileService) MetaKVObserveChildren(metaKVPath *msgrpc.MetaKVPath, stream msgrpc.MobileService_MetaKVObserveChildrenServer) error {

	keyChangedCallback := func(kve metakv.KVEntry) error {

		revStr, err := RevString(kve.Rev)
		if err != nil {
			log.Printf("RunObserveChildren(%s) Error converting rev to a string: Rev: %v. Type(rev): %T, Err: %v", kve.Path, kve.Rev, kve.Rev, err)
			return err
		}

		log.Printf("RunObserveChildren(%s) called back for path %s.  Val: %s.  Rev: %v", metaKVPath.Path, kve.Path, string(kve.Value), revStr)

		metaKvReply := msgrpc.MetaKVPair{
			Path:  kve.Path,
			Rev:   revStr,
			Value: kve.Value,
		}

		return stream.Send(&metaKvReply)
	}

	cancel := make(chan struct{})

	// Blocks indefinitely
	err := metakv.RunObserveChildrenV2(metaKVPath.Path, keyChangedCallback, cancel)

	// Should never get her
	return err

}
