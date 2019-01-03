package mobile_service

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/couchbase/cbauth/service"
	msgrpc "github.com/couchbase/mobile-service/mobile_service_grpc"
	"google.golang.org/grpc"
)

func StartGrpcServer(nodeUuid service.NodeID, grpcListenPort int) {

	var listener net.Listener
	var err error

	// Retry loop to listen on grpc port
	boundListener := false
	for {
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", grpcListenPort))
		if err != nil {
			log.Printf("StartGrpcServer unable to listen on port %d.  Retrying", grpcListenPort)
			time.Sleep(time.Second * 5)
			continue
		}
		log.Printf("StartGrpcServer listening on port: %d", grpcListenPort)
		boundListener = true
		break
	}

	if !boundListener {
		panic(fmt.Sprintf("Could not bind grpc listener"))
	}

	s := grpc.NewServer()

	// Start the MobileService that gateway nodes will connect to
	mobileService := NewMobileService(nodeUuid)
	msgrpc.RegisterMobileServiceServer(s, mobileService)

	// Start the Admin service that an Admin CLI would connect to.
	mobileServiceAdmin := NewMobileServiceAdmin(nodeUuid)
	msgrpc.RegisterMobileServiceAdminServer(s, mobileServiceAdmin)

	// Start the grpc server
	if err := s.Serve(listener); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
