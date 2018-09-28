package mobile_mds

import (
	"github.com/couchbase/cbgt"
	"io"
)

type SgReplicatePIndex struct {

}

// ------- CBGT.dest interface methods -------

func (p *SgReplicatePIndex) Close() error {
	return nil
}

func (p *SgReplicatePIndex) DataUpdate(partition string, key []byte, seq uint64, val []byte,
	cas uint64, extrasType cbgt.DestExtrasType, extras []byte) error {
	return nil
}

func (p *SgReplicatePIndex) DataDelete(partition string, key []byte, seq uint64,
	cas uint64, extrasType cbgt.DestExtrasType, extras []byte) error {
	return nil
}

func (p *SgReplicatePIndex) SnapshotStart(partition string, snapStart, snapEnd uint64) error {
	return nil
}

func (p *SgReplicatePIndex) OpaqueGet(partition string) (value []byte, lastSeq uint64, err error) {
	return nil, 0, nil
}

func (p *SgReplicatePIndex) OpaqueSet(partition string, value []byte) error {
	return nil
}

func (p *SgReplicatePIndex) Rollback(partition string, rollbackSeq uint64) error {
	return nil
}

func (p *SgReplicatePIndex) ConsistencyWait(partition, partitionUUID string,
	consistencyLevel string,
	consistencySeq uint64,
	cancelCh <-chan bool) error {
	return nil
}

func (p *SgReplicatePIndex) Count(pindex *cbgt.PIndex, cancelCh <-chan bool) (uint64, error) {
	return 0, nil
}

func (p *SgReplicatePIndex) Query(pindex *cbgt.PIndex, req []byte, w io.Writer, cancelCh <-chan bool) error {
	return nil
}

func (p *SgReplicatePIndex) Stats(writer io.Writer) error {
	// writer.Write([]byte("{}"))
	return nil
}


