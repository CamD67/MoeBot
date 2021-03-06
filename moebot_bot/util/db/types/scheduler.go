package types

import "time"

type SchedulerType int

type ScheduledOperation struct {
	ID                   int64
	ServerID             int
	Type                 SchedulerType
	PlannedExecutionTime time.Time
}

type ChannelRotation struct {
	ChannelUIDList    []string
	CurrentChannelUID string
	ScheduledOperation
}
