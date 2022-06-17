package types

import (
	"fmt"
)

type PoolId struct {
	int64
}

func (pd PoolId) String() string {
	return fmt.Sprintf("conn_%d", pd.int64)
}

func (pd PoolId) Raw() int64 {
	return pd.int64
}
