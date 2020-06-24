package sfu

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	sdptransform "github.com/notedit/sdp"
	"github.com/pion/ion/pkg/log"
	"github.com/pion/ion/pkg/rtc"
	transport "github.com/pion/ion/pkg/rtc/transport"
	"github.com/pion/webrtc/v2"

	pb "github.com/pion/ion/pkg/proto/sfu"
)

func handleTrickle(r *rtc.Router, t *transport.WebRTCTransport) {
	// for {
	// 	trickle := <-t.GetCandidateChan()
	// 	if trickle != nil {
	// 		broadcaster.Say(proto.SFUTrickleICE, util.Map("mid", t.ID(), "trickle", trickle.ToJSON()))
	// 	} else {
	// 		return
	// 	}
	// }
}

// Publish a stream to the sfu
func (s *server) Publish(ctx context.Context, in *pb.PublishRequest) (*pb.PublishReply, error) {
	log.Infof("publish msg=%v", in)
	if in.Description.Sdp == "" {
		return nil, errors.New("publish: jsep invaild")
	}
	uid := in.Uid
	mid := uuid.New().String()
	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: in.Description.Sdp}

	rtcOptions := transport.RTCOptions{
		Publish: true,
	}

	rtcOptions.Codec = in.Options.Codec
	rtcOptions.Bandwidth = int(in.Options.Bandwidth)
	rtcOptions.TransportCC = in.Options.Transportcc

	videoCodec := strings.ToUpper(rtcOptions.Codec)

	sdpObj, err := sdptransform.Parse(offer.SDP)
	if err != nil {
		log.Errorf("err=%v sdpObj=%v", err, sdpObj)
		return nil, errors.New("publish: sdp parse failed")
	}

	allowedCodecs := make([]uint8, 0)
	tracks := make([]*pb.Track, 0)
	for _, stream := range sdpObj.GetStreams() {
		for id, track := range stream.GetTracks() {
			pt, codecType := getPubPTForTrack(videoCodec, track, sdpObj)

			if len(track.GetSSRCS()) == 0 {
				return nil, errors.New("publish: ssrc not found")
			}
			allowedCodecs = append(allowedCodecs, pt)
			tracks = append(tracks, &pb.Track{Id: id, Sid: stream.GetID(), Ssrc: int32(track.GetSSRCS()[0]), Payload: int32(pt), Type: track.GetMedia(), Codec: codecType})
		}
	}

	rtcOptions.Codecs = allowedCodecs
	pub := transport.NewWebRTCTransport(mid, rtcOptions)
	if pub == nil {
		return nil, errors.New("publish: transport.NewWebRTCTransport failed")
	}

	router := rtc.GetOrNewRouter(mid)

	go handleTrickle(router, pub)

	answer, err := pub.Answer(offer, rtcOptions)
	if err != nil {
		log.Errorf("err=%v answer=%v", err, answer)
		return nil, errors.New("publish: pub.Answer failed")
	}

	router.AddPub(uid, pub)

	log.Infof("publish tracks %v, answer = %v", tracks, answer)

	return &pb.PublishReply{
		Mediainfo: &pb.MediaInfo{Mid: mid},
		Description: &pb.SessionDescription{
			Type: int32(answer.Type),
			Sdp:  answer.SDP,
		},
		Tracks: tracks,
	}, nil
}

// Unpublish a stream
func (s *server) Unpublish(ctx context.Context, in *pb.UnpublishRequest) (*pb.UnpublishReply, error) {
	log.Infof("unpublish msg=%v", in)

	mid := in.Mid
	router := rtc.GetOrNewRouter(mid)
	if router != nil {
		rtc.DelRouter(mid)
		return &pb.UnpublishReply{}, nil
	}
	return nil, errors.New("unpublish: Router not found")
}

// Subscribe to a stream
func (s *server) Subscribe(ctx context.Context, in *pb.SubscribeRequest) (*pb.SubscribeReply, error) {
	log.Infof("subscribe msg=%v", in)
	router := rtc.GetOrNewRouter(in.Mid)
	if router == nil {
		return nil, errors.New("subscribe: router not found")
	}

	if in.Description.Sdp == "" {
		return nil, errors.New("subscribe: unsupported media type")
	}

	sdp := in.Description.Sdp

	rtcOptions := transport.RTCOptions{
		Subscribe: true,
	}

	rtcOptions.Bandwidth = int(in.Options.Bandwidth)
	rtcOptions.TransportCC = in.Options.Transportcc

	subID := uuid.New().String()

	log.Infof("subscribe tracks=%v", in.Tracks)
	rtcOptions.Ssrcpt = make(map[uint32]uint8)

	tracks := make(map[string]*pb.Track)
	for _, track := range in.Tracks {
		rtcOptions.Ssrcpt[uint32(track.Ssrc)] = uint8(track.Payload)
		tracks[track.Sid+" "+track.Id] = track
	}

	sdpObj, err := sdptransform.Parse(sdp)
	if err != nil {
		log.Errorf("err=%v sdpObj=%v", err, sdpObj)
		return nil, errors.New("subscribe: sdp parse failed")
	}

	ssrcPTMap := make(map[int32]uint8)
	allowedCodecs := make([]uint8, 0, len(tracks))

	for _, track := range tracks {
		// Find pt for track given track.Payload and sdp
		ssrcPTMap[track.Ssrc] = getSubPTForTrack(track, sdpObj)
		allowedCodecs = append(allowedCodecs, ssrcPTMap[track.Ssrc])
	}

	// Set media engine codecs based on found pts
	log.Infof("Allowed codecs %v", allowedCodecs)
	rtcOptions.Codecs = allowedCodecs

	// New api
	sub := transport.NewWebRTCTransport(subID, rtcOptions)

	if sub == nil {
		return nil, errors.New("subscribe: transport.NewWebRTCTransport failed")
	}

	go handleTrickle(router, sub)

	for _, track := range tracks {
		ssrc := uint32(track.Ssrc)
		// Get payload type from request track
		pt := uint8(track.Payload)
		if newPt, ok := ssrcPTMap[track.Ssrc]; ok {
			// Override with "negotiated" PT
			pt = newPt
		}

		// I2AacsRLsZZriGapnvPKiKBcLi8rTrO1jOpq c84ded42-d2b0-4351-88d2-b7d240c33435
		//                streamID                        trackID
		streamID := track.Sid
		trackID := track.Id
		log.Infof("AddTrack: codec:%s, ssrc:%d, pt:%d, streamID %s, trackID %s", track.Codec, ssrc, pt, streamID, trackID)
		_, err := sub.AddSendTrack(ssrc, pt, streamID, track.Id)
		if err != nil {
			log.Errorf("err=%v", err)
		}
	}

	// Build answer
	offer := webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: sdp}
	answer, err := sub.Answer(offer, rtcOptions)
	if err != nil {
		log.Errorf("err=%v answer=%v", err, answer)
		return nil, errors.New("unsupported media type")
	}

	router.AddSub(subID, sub)

	log.Infof("subscribe mid %s, answer = %v", subID, answer)
	return &pb.SubscribeReply{
		Mid: subID,
		Description: &pb.SessionDescription{
			Type: int32(answer.Type),
			Sdp:  answer.SDP,
		},
	}, nil
}

// Unsubscribe from a stream
func (s *server) Unsubscribe(ctx context.Context, in *pb.UnsubscribeRequest) (*pb.UnsubscribeReply, error) {
	log.Infof("unsubscribe msg=%v", in)
	mid := in.Mid
	found := false
	rtc.MapRouter(func(id string, r *rtc.Router) {
		subs := r.GetSubs()
		for sid := range subs {
			if sid == mid {
				r.DelSub(mid)
				found = true
				return
			}
		}
	})
	if found {
		return &pb.UnsubscribeReply{}, nil
	}
	return nil, fmt.Errorf("unsubscribe: sub [%s] not found", mid)
}

// func trickle(msg map[string]interface{}) (map[string]interface{}, *nprotoo.Error) {
// 	log.Infof("trickle msg=%v", msg)
// 	router := util.Val(msg, "router")
// 	mid := util.Val(msg, "mid")
// 	//cand := msg["trickle"]
// 	r := rtc.GetOrNewRouter(router)
// 	t := r.GetSub(mid)
// 	if t != nil {
// 		//t.(*transport.WebRTCTransport).AddCandidate(cand)
// 	}

// 	return nil, util.NewNpError(404, "trickle: WebRTCTransport not found!")
// }
