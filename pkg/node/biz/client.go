package biz

import (
	"context"
	"fmt"
	"time"

	nprotoo "github.com/cloudwebrtc/nats-protoo"
	"github.com/pion/ion/pkg/log"
	"github.com/pion/ion/pkg/proto"
	"github.com/pion/ion/pkg/signal"
	"github.com/pion/ion/pkg/util"

	pb "github.com/pion/ion/pkg/proto/sfu"
)

var (
	ridError  = util.NewNpError(codeRoomErr, codeStr(codeRoomErr))
	jsepError = util.NewNpError(codeJsepErr, codeStr(codeJsepErr))
	// sdpError  = util.NewNpError(codeSDPErr, codeStr(codeSDPErr))
	midError = util.NewNpError(codeMIDErr, codeStr(codeMIDErr))
)

// join room
func join(peer *signal.Peer, msg proto.JoinMsg) (interface{}, *nprotoo.Error) {
	log.Infof("biz.join peer.ID()=%s msg=%v", peer.ID(), msg)
	rid := msg.RID

	// Validate
	if msg.RID == "" {
		return nil, ridError
	}

	//already joined this room
	if signal.HasPeer(rid, peer) {
		return emptyMap, nil
	}
	signal.AddPeer(rid, peer)

	islb, found := getRPCForIslb()
	if !found {
		return nil, util.NewNpError(500, "Not found any node for islb.")
	}
	// Send join => islb
	info := msg.Info
	uid := peer.ID()
	_, err := islb.SyncRequest(proto.IslbClientOnJoin, util.Map("rid", rid, "uid", uid, "info", info))
	if err != nil {
		log.Errorf("IslbClientOnJoin failed %v", err.Error())
	}
	// Send getPubs => islb
	islb.AsyncRequest(proto.IslbGetPubs, msg.RoomInfo).Then(
		func(result nprotoo.RawMessage) {
			var resMsg proto.GetPubResp
			if err := result.Unmarshal(&resMsg); err != nil {
				log.Errorf("Unmarshal pub response %v", err)
				return
			}
			log.Infof("IslbGetPubs: result=%v", result)
			for _, pub := range resMsg.Pubs {
				if pub.MID == "" {
					continue
				}
				notif := proto.StreamAddMsg(pub)
				peer.Notify(proto.ClientOnStreamAdd, notif)
			}
		},
		func(err *nprotoo.Error) {})

	return emptyMap, nil
}

func leave(peer *signal.Peer, msg proto.LeaveMsg) (interface{}, *nprotoo.Error) {
	log.Infof("biz.leave peer.ID()=%s msg=%v", peer.ID(), msg)
	defer util.Recover("biz.leave")

	rid := msg.RID

	// Validate
	if msg.RID == "" {
		return nil, ridError
	}

	uid := peer.ID()

	islb, found := getRPCForIslb()
	if !found {
		return nil, util.NewNpError(500, "Not found any node for islb.")
	}

	islb.AsyncRequest(proto.IslbOnStreamRemove, util.Map("rid", rid, "uid", uid))
	_, err := islb.SyncRequest(proto.IslbClientOnLeave, util.Map("rid", rid, "uid", uid))
	if err != nil {
		log.Errorf("IslbOnStreamRemove failed %v", err.Error())
	}
	signal.DelPeer(rid, peer.ID())
	return emptyMap, nil
}

func publish(peer *signal.Peer, msg pb.PublishRequest) (interface{}, *nprotoo.Error) {
	log.Infof("biz.publish peer.ID()=%s", peer.ID())

	nid, sfu, nerr := getRPCForSFU("")
	if nerr != nil {
		log.Warnf("Not found any sfu node, reject: %d => %s", nerr.Code, nerr.Reason)
		return nil, util.NewNpError(nerr.Code, nerr.Reason)
	}

	room := signal.GetRoomByPeer(peer.ID())
	if room == nil {
		return nil, util.NewNpError(codeRoomErr, codeStr(codeRoomErr))
	}

	rid := room.ID()
	uid := peer.ID()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := sfu.Publish(ctx, &msg)

	if err != nil {
		log.Warnf("reject: %s", err)
		return nil, util.NewNpError(500, fmt.Sprintf("%s", err))
	}

	log.Infof("publish: result => %v", result)
	mid := result.Mediainfo.Mid
	tracks := result.Tracks
	islb, found := getRPCForIslb()
	if !found {
		return nil, util.NewNpError(500, "Not found any node for islb.")
	}
	islb.AsyncRequest(proto.IslbOnStreamAdd, util.Map("rid", rid, "nid", nid, "uid", uid, "mid", mid, "tracks", tracks))
	return result, nil
}

// unpublish from app
func unpublish(peer *signal.Peer, msg pb.UnpublishRequest) (interface{}, *nprotoo.Error) {
	log.Infof("signal.unpublish peer.ID()=%s msg=%v", peer.ID(), msg)

	mid := msg.Mid
	uid := peer.ID()

	_, sfu, nerr := getRPCForSFU(mid)
	if nerr != nil {
		log.Warnf("Not found any sfu node, reject: %d => %s", nerr.Code, nerr.Reason)
		return nil, nerr
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := sfu.Unpublish(ctx, &pb.UnpublishRequest{Mid: string(mid)})
	if err != nil {
		log.Warnf("reject: %s", err)
		return nil, util.NewNpError(500, fmt.Sprintf("%s", err))
	}

	islb, found := getRPCForIslb()
	if !found {
		return nil, util.NewNpError(500, "Not found any node for islb.")
	}
	// if this mid is a webrtc pub
	// tell islb stream-remove, `rtc.DelPub(mid)` will be done when islb broadcast stream-remove
	islb.AsyncRequest(proto.IslbOnStreamRemove, util.Map("uid", uid, "mid", mid))
	return emptyMap, nil
}

func subscribe(peer *signal.Peer, msg pb.SubscribeRequest) (interface{}, *nprotoo.Error) {
	log.Infof("biz.subscribe peer.ID()=%s ", peer.ID())
	mid := msg.Mid

	// Validate
	if mid == "" {
		return nil, midError
	} else if msg.Description.Sdp == "" {
		return nil, jsepError
	}

	nodeID, sfu, nerr := getRPCForSFU(mid)
	if nerr != nil {
		log.Warnf("Not found any sfu node, reject: %d => %s", nerr.Code, nerr.Reason)
		return nil, util.NewNpError(nerr.Code, nerr.Reason)
	}

	// TODO:
	if nodeID != "node for mid" {
		log.Warnf("Not the same node, need to enable sfu-sfu relay!")
	}

	room := signal.GetRoomByPeer(peer.ID())
	rid := room.ID()

	jsep := msg.Description

	islb, found := getRPCForIslb()
	if !found {
		return nil, util.NewNpError(500, "Not found any node for islb.")
	}

	minfo, nerr := islb.SyncRequest(proto.IslbGetMediaInfo, proto.MediaInfo{RID: rid, MID: proto.MID(mid)})
	if nerr != nil {
		log.Warnf("reject: %d => %s", nerr.Code, nerr.Reason)
		return nil, util.NewNpError(nerr.Code, nerr.Reason)
	}
	var some map[string]interface{}
	if err := minfo.Unmarshal(&some); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	res, err := sfu.Subscribe(ctx, &pb.SubscribeRequest{
		Mid: string(mid),
		Description: &pb.SessionDescription{
			Type: int32(jsep.Type),
			Sdp:  jsep.Sdp,
		},
		Tracks: some["tracks"],
	})

	if err != nil {
		log.Warnf("reject: %s", err)
		return nil, util.NewNpError(500, fmt.Sprintf("%s", err))
	}

	log.Infof("subscribe: result => %v", res)
	return res, nil
}

func unsubscribe(peer *signal.Peer, msg pb.UnsubscribeRequest) (interface{}, *nprotoo.Error) {
	log.Infof("biz.unsubscribe peer.ID()=%s msg=%v", peer.ID(), msg)
	mid := msg.Mid

	// Validate
	if mid == "" {
		return nil, midError
	}

	_, sfu, nerr := getRPCForSFU(mid)
	if nerr != nil {
		log.Warnf("Not found any sfu node, reject: %d => %s", nerr.Code, nerr.Reason)
		return nil, util.NewNpError(nerr.Code, nerr.Reason)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result, err := sfu.Unsubscribe(ctx, &msg)
	if err != nil {
		log.Warnf("reject: %s", err)
		return nil, util.NewNpError(500, fmt.Sprintf("%s", err))
	}

	log.Infof("publish: result => %v", result)
	return result, nil
}

func broadcast(peer *signal.Peer, msg proto.BroadcastMsg) (interface{}, *nprotoo.Error) {
	log.Infof("biz.broadcast peer.ID()=%s msg=%v", peer.ID(), msg)

	// Validate
	if msg.RID == "" || msg.UID == "" {
		return nil, ridError
	}

	islb, found := getRPCForIslb()
	if !found {
		return nil, util.NewNpError(500, "Not found any node for islb.")
	}
	rid, uid, info := msg.RID, msg.UID, msg.Info
	islb.AsyncRequest(proto.IslbOnBroadcast, util.Map("rid", rid, "uid", uid, "info", info))
	return emptyMap, nil
}

func trickle(peer *signal.Peer, msg proto.TrickleMsg) (interface{}, *nprotoo.Error) {
	log.Infof("biz.trickle peer.ID()=%s msg=%v", peer.ID(), msg)
	mid := msg.MID

	// Validate
	if msg.RID == "" || msg.UID == "" {
		return nil, ridError
	}

	_, sfu, err := getRPCForSFU(mid)
	if err != nil {
		log.Warnf("Not found any sfu node, reject: %d => %s", err.Code, err.Reason)
		return nil, util.NewNpError(err.Code, err.Reason)
	}

	trickle := msg.Trickle

	sfu.AsyncRequest(proto.ClientTrickleICE, util.Map("mid", mid, "trickle", trickle))
	return emptyMap, nil
}
