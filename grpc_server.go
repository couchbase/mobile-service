package mobile_mds

import (
	"fmt"
	"io"
	"log"
	"net"

	"context"

	"github.com/couchbase/mobile-mds/mobile_service"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"github.com/couchbase/cbauth/metakv"
	"time"
	"github.com/couchbase/cbauth/service"
	"github.com/couchbase/cbauth"
	"encoding/json"
	"bytes"
	"strconv"
	"strings"
)

type server struct{

	NodeUuid service.NodeID

	LastSeenGateways *LastSeenMap

}

func NewServer(nodeUuid service.NodeID) *server {

	s := server{
		NodeUuid: nodeUuid,
		LastSeenGateways: NewLastSeenMap(),
	}

	go s.DeleteStaleGatewayEntries()

	return &s
}

func (s *server) DeleteStaleGatewayEntries()  {

	for {

		log.Printf("Checking for stale entries in s.LastSeenGateways.StaleEntries")

		staleEntries := s.LastSeenGateways.StaleEntries(time.Second * 30)

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


func (s *server) SendStats(stream mobile_service.MobileService_SendStatsServer) error {
	for {
		stats, err := stream.Recv()

		log.Printf("GrpcServer SendStats() stream received stats: %v.", stats)

		if err != nil {
			// TODO: There should be a pre-existing association of the stream with the nodeuuid
			// TODO: and then at this point it could lookup the node id based on the stream, and immediately
			// TODO: remove presence status from MetaKV.  (it will still get cleaned up w/o this, but this
			// TODO: would decrease latency)
			log.Printf("GrpcServer SendStats() received err: %v.", err)
			return err
		}

		// Authentication
		creds, err := cbauth.Auth(stats.Creds.Username, stats.Creds.Password)
		if err != nil {
			log.Printf("GrpcServer auth err: %v.", err)
			continue
		}

		// Check permissions
		allowed, err := creds.IsAllowed("cluster.settings!write")
		if err != nil {
			log.Printf("GrpcServer auth err checking permissions: %v.", err)
			continue
		}
		if !allowed {
			log.Printf("GrpcServer permission denied")
			continue
		}


		if err := s.RecordClientHeartbeat(stats); err != nil {
			log.Printf("Error recording client heartbeat: %v", err)
		}
		if err == io.EOF {
			return stream.SendAndClose(&mobile_service.StatsReply{})
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *server) RecordClientHeartbeat(stats *mobile_service.Stats) error {

	keypath := fmt.Sprintf("/mobile/state/%s/%s", s.NodeUuid, stats.Gateway.Uuid)

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

func (s *server) MetaKVGet(context context.Context, metaKVPath *mobile_service.MetaKVPath) (*mobile_service.MetaKVPair, error) {

	val, rev, err := metakv.Get(metaKVPath.Path)
	if err != nil {
		return &mobile_service.MetaKVPair{}, err
	}

	revStr, err := RevString(rev)
	if err != nil {
		return &mobile_service.MetaKVPair{}, err
	}
	log.Printf("Rev marshalled: %s", string(revStr))

	return &mobile_service.MetaKVPair{
		Value: val,
		Rev: revStr,
		Path: metaKVPath.Path,
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


func (s *server) MetaKVSet(context context.Context, metaKVPair *mobile_service.MetaKVPair) (*mobile_service.Empty, error) {

	log.Printf("Updating key pair: %+v", metaKVPair)

	// TODO: pass in rev from the metaKVPair.  Running into errors when trying to go from interface{} -> string -> interface{}
	if err := metakv.Set(metaKVPair.Path, []byte(metaKVPair.Value), nil); err != nil {

		log.Printf("Error updating key pair: %+v.  Err: %v", metaKVPair, err)

		return &mobile_service.Empty{}, err
	}

	log.Printf("Updated key pair: %+v", metaKVPair)


	return &mobile_service.Empty{}, nil

}


func (s *server) MetaKVAdd(context context.Context, metaKVPair *mobile_service.MetaKVPair) (*mobile_service.Empty, error) {


	if err := metakv.Add(metaKVPair.Path, []byte(metaKVPair.Value)); err != nil {
		return &mobile_service.Empty{}, err
	}

	return &mobile_service.Empty{}, nil

}

func (s *server) MetaKVDelete(context context.Context, metaKVPair *mobile_service.MetaKVPair) (*mobile_service.Empty, error) {

	log.Printf("Deleting key pair: %+v", metaKVPair)

	// TODO: pass in rev from the metaKVPair.  Running into errors when trying to go from interface{} -> string -> interface{}
	if err := metakv.Delete(metaKVPair.Path, nil); err != nil {
		log.Printf("Error deleting key pair: %+v.  Err: %v", metaKVPair, err)
		return &mobile_service.Empty{}, err
	}

	log.Printf("Deleted key pair: %+v", metaKVPair)


	return &mobile_service.Empty{}, nil

}


func (s *server) MetaKVRecursiveDelete(context context.Context, metaKVPath *mobile_service.MetaKVPath) (*mobile_service.Empty, error) {
	if err := metakv.RecursiveDelete(metaKVPath.Path); err != nil {
		log.Printf("Error deleting key path: %+v.  Err: %v", metaKVPath, err)
		return &mobile_service.Empty{}, err
	}
	return &mobile_service.Empty{}, nil
}


func (s *server) MetaKVListAllChildren(context context.Context, metaKVPath *mobile_service.MetaKVPath) (*mobile_service.MetaKVPairs, error) {

	entries, err := metakv.ListAllChildren(metaKVPath.Path)
	if err != nil {
		return &mobile_service.MetaKVPairs{}, err
	}

	items := make([]*mobile_service.MetaKVPair, len(entries))
	for i, entry := range entries {

		revStr, err := RevString(entry.Rev)
		if err != nil {
			log.Printf("Error converting rev to string: %v", err)
		}

		items[i] = &mobile_service.MetaKVPair{
			Path: entry.Path,
			Rev: revStr,
			Value: entry.Value,
		}
	}

	metakvPairs := mobile_service.MetaKVPairs{
		Items: items,
	}

	return &metakvPairs, nil

}

func (s *server) MetaKVObserveChildren(metaKVPath *mobile_service.MetaKVPath, stream mobile_service.MobileService_MetaKVObserveChildrenServer) (error) {

	keyChangedCallback := func(path string, value []byte, rev interface{}) error {

		revStr, err := RevString(rev)
		if err != nil {
			log.Printf("RunObserveChildren(%s) Error converting rev to a string: Rev: %v. Type(rev): %T, Err: %v", path, rev, rev, err)
			return err
		}

		log.Printf("RunObserveChildren(%s) called back for path %s.  Val: %s.  Rev: %v", metaKVPath.Path, path, string(value), revStr)


		metaKvReply := mobile_service.MetaKVPair{
			Path: path,
			Rev: revStr,
			Value: value,
		}

		return stream.Send(&metaKvReply)
	}

	cancel := make(chan struct{})

	//go func() {
	//	if err := metakv.RunObserveChildren(metaKVPath.Path, keyChangedCallback, cancel); err != nil {
	//		panic(fmt.Sprintf("Error calling RunObserveChildren(): %v", err))
	//	}
	//}()

	// Blocks indefinitely
	err := metakv.RunObserveChildren(metaKVPath.Path, keyChangedCallback, cancel)

	// Should never get her
	return err

}


func StartGrpcServer(nodeUuid service.NodeID, grpcListenPort int) {

	var listener net.Listener
	var err error
	startPort := grpcListenPort

	boundListener := false
	for port := startPort; port < (startPort + 1000); port++ {
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			log.Printf("StartGrpcServer unable to listen on port %d.  Trying port %d", port, port+1)
			continue
		}
		log.Printf("StartGrpcServer listening on port: %d", port)
		boundListener = true
		break
	}

	if !boundListener {
		panic(fmt.Sprintf("Could not bind grpc listener"))
	}

	s := grpc.NewServer()

	mobileService := NewServer(nodeUuid)

	mobile_service.RegisterMobileServiceServer(s, mobileService)

	// Register reflection service on gRPC server.
	reflection.Register(s)
	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}



func DiscoverRev(rev interface{}) {

	log.Printf("Type of rev: %T", rev)

	revBytesTest4, ok := rev.([]byte)
	if ok {
		log.Printf("Rev as string4: %v", string(revBytesTest4)  )
	} else {
		log.Printf("not a []byte")
	}

	revBytesTest, ok := rev.([]uint8)
	if ok {
		log.Printf("Rev as string: %v or %v", bytes.NewBuffer(revBytesTest).String(), string(revBytesTest) )
	} else {
		log.Printf("not a []uint8")
	}

	revBytesTest2, ok := rev.(*[]uint8)
	if ok {
		log.Printf("Rev as string2: %v or %v", bytes.NewBuffer(*revBytesTest2).String(), string(*revBytesTest2) )
	} else {
		log.Printf("not a *[]uint8")
	}


	revBytesTest3, ok := rev.(*[]byte)
	if ok {
		log.Printf("Rev as string3: %v or %v", bytes.NewBuffer(*revBytesTest3).String(), string(*revBytesTest3) )
	} else {
		log.Printf("not a *[]byte")
	}

}