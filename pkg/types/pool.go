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

func (pd *PoolId) SetDirection(dire int32) {
	pd.Direction = dire
}

func (pd *PoolId) String() string {
	return fmt.Sprintf("conn_%d_%d", pd.Value, pd.Direction)
}

func (pd *PoolId) Raw() int64 {
	return pd.Value
}
