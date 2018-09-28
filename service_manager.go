package mobile_mds

import (
	"github.com/couchbase/cbauth/service"
	"log"
)

type ServiceManager struct {
	NodeInfo *service.NodeInfo
}

func NewServiceManager(nodeId service.NodeID) *ServiceManager {
	return &ServiceManager{
		NodeInfo: &service.NodeInfo{
			NodeID: nodeId,
			Priority: 0,
			Opaque: nil,
		},
	}
}

func (s *ServiceManager) GetNodeInfo() (*service.NodeInfo, error) {
	// TODO: implement
	log.Printf("ServiceManager.GetNodeInfo() called")
	return s.NodeInfo, nil
}

func (s *ServiceManager) Shutdown() error {
	// TODO: implement
	log.Printf("ServiceManager.Shutdown() called")
	return nil
}

func (s *ServiceManager) GetTaskList(rev service.Revision, cancel service.Cancel) (*service.TaskList, error) {
	// TODO: implement
	log.Printf("ServiceManager.Shutdown() called with rev: %+v", rev)
	return NewTaskList(rev), nil
}

func (s *ServiceManager) CancelTask(id string, rev service.Revision) error {
	// TODO: implement
	log.Printf("ServiceManager.CancelTask() called with id: %v, rev: %+v", id, rev)
	return nil
}

func (s *ServiceManager) GetCurrentTopology(rev service.Revision, cancel service.Cancel) (*service.Topology, error) {
	log.Printf("ServiceManager.GetCurrentTopology() called rev: %+v", rev)
	defer log.Printf("/ServiceManager.GetCurrentTopology() returning")

	// Experiment: block forever and see what happens
	select {}

	serviceTopology := service.Topology{
		Rev: rev,  // TODO: Is this right?  What should this be using?
		Nodes: []service.NodeID{s.NodeInfo.NodeID},  // TODO: fix.  for now, just include "this" node in list.  
		IsBalanced: false,  // TODO: what should this be?
		Messages: []string{},
	}
	
	return &serviceTopology, nil
}

func (s *ServiceManager) PrepareTopologyChange(change service.TopologyChange) error {
	// TODO: implement
	log.Printf("ServiceManager.PrepareTopologyChange() called with change: %+v", change)
	return nil
}

func (s *ServiceManager) StartTopologyChange(change service.TopologyChange) error {
	// TODO: implement
	log.Printf("ServiceManager.PrepareTopologyChange() called with change: %+v", change)
	return nil
}


