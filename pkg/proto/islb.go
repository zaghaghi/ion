package proto

import pb "github.com/pion/ion/pkg/proto/sfu"

type PubInfo struct {
	MediaInfo
	Info   ClientUserInfo `json:"info"`
	Stream pb.Stream      `json:"stream"`
}

type GetPubResp struct {
	RoomInfo
	Pubs []PubInfo
}

type GetMediaParams struct {
	RID RID
	MID MID
}

type FindServiceParams struct {
	Service string
	MID     MID
}

type GetSFURPCParams struct {
	RPCID   string
	EventID string
	ID      string
	Name    string
	Service string
	GRPC    string
}
