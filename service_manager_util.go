package mobile_mds

import "github.com/couchbase/cbauth/service"

// The functions/helpers defined here should be moved into the cbauth/service repo

func NewTaskList(rev service.Revision) *service.TaskList {
	taskList := service.TaskList{
		Rev: rev,
		Tasks: []service.Task{},
	}
	return &taskList
}
