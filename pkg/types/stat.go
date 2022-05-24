package types

import "time"

type Status struct {
	StartTime    time.Time
	TargetId     string
	ImplType     int32
	PairId       string
	ParentPairId string
}
