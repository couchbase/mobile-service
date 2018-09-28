package mobile_mds

import (
	"github.com/couchbase/cbgt"
	"io"
)

type ConvergenceImportPIndex struct {

}

// ------- CBGT.dest interface methods -------

func (p *ConvergenceImportPIndex) Close() error {
	return nil 
}

func (p *ConvergenceImportPIndex) DataUpdate(vbucketNo string, key []byte, seq uint64, val []byte,
	cas uint64, extrasType cbgt.DestExtrasType, extras []byte) error {
	return nil
}

func (p *ConvergenceImportPIndex) DataDelete(vbucketNo string, key []byte, seq uint64,
	cas uint64, extrasType cbgt.DestExtrasType, extras []byte) error {
	return nil
}

func (p *ConvergenceImportPIndex) SnapshotStart(vbucketNo string, snapStart, snapEnd uint64) error {
	return nil
}

func (p *ConvergenceImportPIndex) OpaqueGet(vbucketNo string) (value []byte, lastSeq uint64, err error) {
	return nil, 0, nil
}

func (p *ConvergenceImportPIndex) OpaqueSet(vbucketNo string, value []byte) error {
	return nil
}

func (p *ConvergenceImportPIndex) Rollback(vbucketNo string, rollbackSeq uint64) error {
	return nil
}

func (p *ConvergenceImportPIndex) ConsistencyWait(vbucketNo, partitionUUID string,
	consistencyLevel string,
	consistencySeq uint64,
	cancelCh <-chan bool) error {
	return nil 
}

func (p *ConvergenceImportPIndex) Count(pindex *cbgt.PIndex, cancelCh <-chan bool) (uint64, error) {
	return 0, nil
}

func (p *ConvergenceImportPIndex) Query(pindex *cbgt.PIndex, req []byte, w io.Writer, cancelCh <-chan bool) error {
	return nil
}

func (p *ConvergenceImportPIndex) Stats(writer io.Writer) error {
	// writer.Write([]byte("{}"))
	return nil
}

