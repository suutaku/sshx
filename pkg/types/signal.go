package types

type SignalingInfo struct {
	Flag              int    `json:"flag"`
	Source            string `json:"source"`
	SDP               string `json:"sdp"`
	Candidate         []byte `json:"candidate"`
	ID                int64  `json:"id"`
	Target            string `json:"target"`
	PeerType          int32  `json:"peer_type"`
	RemoteRequestType int32  `json:"remote_request_type"`
}
