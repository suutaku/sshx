package types

import (
	"fmt"
)

type PoolId struct {
	Value     int64
	Direction int32
}

func NewPoolId(id int64) *PoolId {
	return &PoolId{
		Value: id,
	}
}

func (pd *PoolId) String(direct int32) string {
	return fmt.Sprintf("conn_%d_%d", pd.Value, direct)
}

func (pd *PoolId) Raw() int64 {
	return pd.Value
}
