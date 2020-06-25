package proto

import media "github.com/pion/ion/pkg/proto/media"

type PubInfo struct {
	MediaInfo
	Info   ClientUserInfo `json:"info"`
	Stream media.Stream   `json:"stream"`
}

type GetPubResp struct {
	RoomInfo
	Pubs []PubInfo
}

type GetMediaParams struct {
	RID string
	MID string
}

type FindServiceParams struct {
	Service string
	MID     string
}

type GetSFURPCParams struct {
	RPCID   string
	EventID string
	ID      string
	Name    string
	Service string
	GRPC    string
}
