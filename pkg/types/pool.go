package types

import (
	"fmt"
)

type PoolId struct {
	Value     int64
	Direction int32
	ImplCode  int32
}

func NewPoolId(id int64, impc int32) *PoolId {
	return &PoolId{
		Value:    id,
		ImplCode: impc,
	}
}

func (pd *PoolId) String(direct int32) string {
	return fmt.Sprintf("conn_%d_%d_%d", pd.ImplCode, pd.Value, direct)
}

func (pd *PoolId) Raw() int64 {
	return pd.Value
}
