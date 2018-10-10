package mobile_mds

import (
	"github.com/couchbase/cbauth/service"
	"log"
	"encoding/binary"
	"os"
)

/**

NOT WORKING -- See ServiceManager2.go

 */

type ServiceManager struct {
	NodeInfo *service.NodeInfo
	KnownServers []service.NodeID
	State State
}

type State struct {
	Rev    uint64
	RebalanceID   string
	RebalanceTask *service.Task
}

func NewServiceManager(nodeId service.NodeID) *ServiceManager {

	opaque := struct {
		Host string `json:"host"`
	}{
		"localhost", // TODO
	}

	nodeInfo := &service.NodeInfo{
		NodeID: nodeId,
		Priority: 0,
		Opaque: opaque,
	}

	sm := &ServiceManager{
		NodeInfo: nodeInfo,
		KnownServers: []service.NodeID{nodeId},
		State: State{
			Rev: 0,
			RebalanceID: "",
		},
	}

	log.Printf("NewServiceManager: %+v", sm)

	return sm
}

// Return the NodeInfo associated with this service
func (s *ServiceManager) GetNodeInfo() (*service.NodeInfo, error) {
	log.Printf("ServiceManager.GetNodeInfo() called, returning: %+v", s.NodeInfo)
	return s.NodeInfo, nil
}

// This is invoked before the process is killed, and gives a chance for the process to do any cleanup required.
func (s *ServiceManager) Shutdown() error {
	// TODO: implement
	log.Printf("Shutdown() called, calling os.Exit()")

	os.Exit(0)

	return nil
}

// Return the list of in progress tasks related to rebalance.
// Only invoked on the node where StartTopologyChange() was called.  (TODO: verify this)
func (s *ServiceManager) GetTaskList(rev service.Revision, cancel service.Cancel) (*service.TaskList, error) {
	// TODO: implement
	log.Printf("ServiceManager.Shutdown() called with rev: %+v", rev)
	return NewTaskList(EncodeRev(s.State.Rev)), nil
}

// Cancel a task in progress related to rebalance.  An example of when this is invoked is when
// a user cancels a rebalance operation from the UI.
// Only invoked on the node where StartTopologyChange() was called.
func (s *ServiceManager) CancelTask(id string, rev service.Revision) error {
	// TODO: implement
	log.Printf("ServiceManager.CancelTask() called with id: %v, rev: %+v", id, rev)
	return nil
}

// Services are authoritative in terms of declaring their topology.  Typically this matches what
// is specified by the user via the admin UI, but services have the ability to override this.
//
// This is only invoked on the leader node.  (TODO: verify this)
//
// The rev parameter passed in is similar to an HTTP ETag.
// Typically a service will:
//    - If the service doesn't know about any previous rev parameters, immediately
//      return currently known topology and store the rev.
//    - If the topology is known to have changed since the last time
//      it was called with this rev, immediately return the updated topology
//    - If the topology hasn't changed, block (potentially forever) until something
//      triggers the topology to change, and then return the updated topology
func (s *ServiceManager) GetCurrentTopology(rev service.Revision, cancel service.Cancel) (serviceTopology *service.Topology, err error) {
	log.Printf("ServiceManager.GetCurrentTopology() called rev: %+v", rev)

	serviceTopology = &service.Topology{
		Rev: EncodeRev(s.State.Rev),
		Nodes: s.KnownServers,
		IsBalanced: true,
		Messages: []string{},
	}

	log.Printf("ServiceManager.GetCurrentTopology() returning: %+v", serviceTopology)
	
	return serviceTopology, nil
}

// Invoked on all nodes prior to topology change.  This should return immediately, and any work
// kicked off in reaction to this should happen asynchronously.
func (s *ServiceManager) PrepareTopologyChange(change service.TopologyChange) error {
	// TODO: implement
	log.Printf("ServiceManager.PrepareTopologyChange() called with change: %+v", change)
	return nil
}

// Invoked on the leader node to trigger any work required for rebalances.
// This should return immediately, and any work kicked off in reaction to this should happen asynchronously.
func (s *ServiceManager) StartTopologyChange(change service.TopologyChange) error {

	log.Printf("ServiceManager.StartTopologyChange() called with change: %+v", change)

	// TODO: implement

	s.State.Rev = DecodeRev(change.CurrentTopologyRev)

	s.KnownServers = []service.NodeID{
		s.NodeInfo.NodeID,
	}

	for _, node := range change.KeepNodes {
		s.KnownServers = append(s.KnownServers, node.NodeInfo.NodeID)
	}

	return nil
}


func EncodeRev(rev uint64) service.Revision {
	ext := make(service.Revision, 8)
	binary.BigEndian.PutUint64(ext, rev)

	return ext
}

func DecodeRev(ext service.Revision) uint64 {
	return binary.BigEndian.Uint64(ext)
}
