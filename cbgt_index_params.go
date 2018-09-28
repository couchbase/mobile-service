package mobile_mds

import (
	"fmt"

	"github.com/couchbase/cbgt"
)

type CbgtIndexParams struct {
	
	IndexType MobileServiceIndexType

	BucketName string

	IndexName string

	// The number of shards the total number of vbuckets should be sharded into
	NumShards uint16
	
}

func (p *CbgtIndexParams) GetPlanParams(numVbucketsTotal uint16) cbgt.PlanParams {

	IsPowerOfTwo := func(n uint16) bool {
		return (n & (n - 1)) == 0
	}

	// Make sure the number of vbuckets is a power of two, since it's possible
	// (but not common) to configure the number of vbuckets as such.
	if !IsPowerOfTwo(numVbucketsTotal) {
		panic(fmt.Sprintf("The number of vbuckets is not a power of two: %d", numVbucketsTotal))
	}

	// We can't allow more shards than vbuckets, that makes no sense because each
	// shard would be responsible for less than one vbucket.
	if p.NumShards > numVbucketsTotal {
		panic(fmt.Sprintf("The number of shards (%v) must be less than the number of vbuckets (%v)", p.NumShards, numVbucketsTotal))
	}

	// Calculate numVbucketsPerShard based on numVbuckets and num_shards.
	// Due to the guarantees above and the ValidateOrPanic() method, this
	// is guaranteed to divide evenly.
	numVbucketsPerShard := numVbucketsTotal / p.NumShards

	return cbgt.PlanParams{
		MaxPartitionsPerPIndex: int(numVbucketsPerShard),
		NumReplicas:            0, // no use case for pindex replicas
	}

}
