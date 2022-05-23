package types

const (
	OPTION_TYPE_UP = iota
	OPTION_TYPE_DOWN
	OPTION_TYPE_STAT
)

const (
	APP_TYPE_SSH = iota
	APP_TYPE_VNC
	APP_TYPE_SCP
	APP_TYPE_SFS
	APP_TYPE_PROXY
	APP_TYPE_STAT
	APP_TYPE_VNC_SERVICE
)

// some signaling request type
const (
	SIG_TYPE_UNKNOWN = iota
	SIG_TYPE_CANDIDATE
	SIG_TYPE_ANSWER
	SIG_TYPE_OFFER
)
