package conf

type ConnectInfo struct {
	Flag      int    `json:"flag"`
	Source    string `json:"source"`
	SDP       string `json:"sdp"`
	Candidate []byte `json:"candidate"`
	ID        int64  `json:"id"`
	Timestamp int64  `json:"timestamp"`
	Type      int    `json:"type"`
}
