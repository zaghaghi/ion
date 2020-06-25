package islb

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	nprotoo "github.com/cloudwebrtc/nats-protoo"
	"github.com/pion/ion/pkg/discovery"
	"github.com/pion/ion/pkg/log"
	"github.com/pion/ion/pkg/proto"
	"github.com/pion/ion/pkg/util"

	pb "github.com/pion/ion/pkg/proto/islb"
	media "github.com/pion/ion/pkg/proto/media"
)

// WatchServiceNodes .
func WatchServiceNodes(service string, state discovery.NodeStateType, node discovery.Node) {
	id := node.ID
	if state == discovery.UP {
		if _, found := services[id]; !found {
			services[id] = node
			service := node.Info["service"]
			name := node.Info["name"]
			log.Debugf("Service [%s] UP %s => %s", service, name, id)
		}
	} else if state == discovery.DOWN {
		if _, found := services[id]; found {
			service := node.Info["service"]
			name := node.Info["name"]
			log.Debugf("Service [%s] DOWN %s => %s", service, name, id)
			if service == "sfu" {
				removeStreamsByNode(node.Info["id"])
			}
			delete(services, id)
		}
	}
}

func removeStreamsByNode(nodeID string) {
	log.Infof("removeStreamsByNode: node => %s", nodeID)
	mkey := media.Info{
		Dc:  dc,
		Nid: nodeID,
	}.BuildKey()
	for _, key := range redis.Keys(mkey + "*") {
		log.Infof("streamRemove: key => %s", key)
		minfo, err := media.ParseInfoKey(key)
		if err == nil {
			log.Warnf("TODO: Internal Server Error (500) to %s", minfo.Uid)
		}
		err = redis.Del(key)
		if err != nil {
			log.Errorf("redis.Del err = %v", err)
		}
	}
}

// WatchAllStreams .
func WatchAllStreams() {
	mkey := media.Info{
		Dc: dc,
	}.BuildKey()
	log.Infof("Watch all streams, mkey = %s", mkey)
	for _, key := range redis.Keys(mkey) {
		log.Infof("Watch stream, key = %s", key)
		watchStream(key)
	}
}

func watchStream(key string) {
	log.Infof("Start watch stream: key = %s", key)
	go func() {
		for cmd := range redis.Watch(context.TODO(), key) {
			log.Infof("watchStream: key %s cmd %s modified", key, cmd)
			if cmd == "del" || cmd == "expired" {
				// dc1/room1/media/pub/74baff6e-b8c9-4868-9055-b35d50b73ed6#LUMGUQ
				info, err := media.ParseInfoKey(key)
				if err == nil {
					msg := util.Map("dc", info.Dc, "rid", info.Rid, "uid", info.Uid, "mid", info.Mid)
					log.Infof("watchStream.BroadCast: onStreamRemove=%v", msg)
					broadcaster.Say(proto.IslbOnStreamRemove, msg)
				}
				//Stop watch loop after key removed.
				break
			}
		}
		log.Infof("Stop watch stream: key = %s", key)
	}()
}

/* FindService nodes by name, such as sfu|mcu|sip-gateway|rtmp-gateway */
func (s *server) FindService(ctx context.Context, in *pb.FindServiceRequest) (*pb.FindServiceReply, error) {
	service := in.Service
	mid := in.Mid

	if mid != "" {
		mkey := media.Info{
			Dc:  dc,
			Mid: mid,
		}.BuildKey()
		log.Infof("Find mids by mkey %s", mkey)
		for _, key := range redis.Keys(mkey + "*") {
			log.Infof("Got: key => %s", key)
			minfo, err := media.ParseInfoKey(key)
			if err != nil {
				break
			}
			for _, node := range services {
				name := node.Info["name"]
				id := node.Info["id"]
				if service == node.Info["service"] && minfo.Nid == id {
					grpc := discovery.GetGRPCAddress(node)
					log.Infof("findServiceNode: by node ID %s, [%s] %s => %s", minfo.Nid, service, name, grpc)
					return &pb.FindServiceReply{
						Id:      id,
						Name:    name,
						Service: service,
						Grpc:    grpc,
					}, nil
				}
			}
		}
	}

	// TODO: Add a load balancing algorithm.
	for _, node := range services {
		if service == node.Info["service"] {
			rpcID := discovery.GetRPCChannel(node)
			name := node.Info["name"]
			id := node.Info["id"]
			grpc := discovery.GetGRPCAddress(node)
			log.Infof("findServiceNode: [%s] %s => %s", service, name, rpcID)
			return &pb.FindServiceReply{
				Id:      id,
				Name:    name,
				Service: service,
				Grpc:    grpc,
			}, nil
		}
	}

	return nil, fmt.Errorf("service node [%s] not found", service)
}

func (s *server) StreamAdd(ctx context.Context, in *pb.StreamAddRequest) (*pb.StreamAddReply, error) {
	log.Debugf("streamAdd: => %+v", in)
	ukey := pb.UserInfo{
		Dc:  dc,
		Rid: in.Minfo.Rid,
		Uid: in.Minfo.Uid,
	}.BuildKey()

	in.Minfo.Dc = dc
	mkey := in.Minfo.BuildKey()

	field, value, err := pb.NodeInfo{
		Name: nid,
		Id:   nid,
		Type: "origin",
	}.MarshalNodeField()
	if err != nil {
		log.Errorf("Set: %v ", err)
	}
	err = redis.HSetTTL(mkey, field, value, redisLongKeyTTL)
	if err != nil {
		log.Errorf("Set: %v ", err)
	}

	field, value, err = in.Stream.MarshalTracks()
	if err != nil {
		log.Errorf("MarshalTrackField: %v ", err)
	}

	log.Infof("SetTrackField: mkey, field, value = %s, %s, %s", mkey, field, value)
	err = redis.HSetTTL(mkey, field, value, redisLongKeyTTL)
	if err != nil {
		log.Errorf("redis.HSetTTL err = %v", err)
	}

	// dc1/room1/user/info/${uid} info {"name": "Guest"}
	fields := redis.HGetAll(ukey)

	extraInfo := &pb.UserMetadata{}
	if infoStr, ok := fields["info"]; ok {
		if err := json.Unmarshal([]byte(infoStr), extraInfo); err != nil {
			log.Errorf("Unmarshal pub extra info %v", err)
			extraInfo = in.Uinfo
		}
		in.Uinfo = extraInfo
	}

	log.Infof("Broadcast: [stream-add] => %v", in)
	broadcaster.Say(proto.IslbOnStreamAdd, in)

	watchStream(mkey)
	return &pb.StreamAddReply{}, nil
}

// func streamRemove(data proto.StreamRemoveMsg) (map[string]interface{}, *nprotoo.Error) {
func (s *server) StreamRemove(ctx context.Context, in *pb.StreamRemoveRequest) (*pb.StreamRemoveReply, error) {
	in.Minfo.Dc = dc
	mkey := in.Minfo.BuildKey()

	log.Infof("streamRemove: key => %s", mkey)
	for _, key := range redis.Keys(mkey + "*") {
		log.Infof("streamRemove: key => %s", key)
		err := redis.Del(key)
		if err != nil {
			log.Errorf("redis.Del err = %v", err)
		}
	}
	return &pb.StreamRemoveReply{}, nil
}

func (s *server) GetPubs(ctx context.Context, in *pb.GetPubsRequest) (*pb.GetPubsReply, error) {
	rid := in.Rid
	key := media.Info{
		Dc:  dc,
		Rid: rid,
	}.BuildKey()
	log.Infof("getPubs: root key=%s", key)

	var pubs []*pb.Pub
	for _, path := range redis.Keys(key + "*") {
		log.Infof("getPubs media info path = %s", path)
		info, err := media.ParseInfoKey(path)
		if err != nil {
			log.Errorf("%v", err)
		}
		fields := redis.HGetAll(pb.UserInfo{
			Dc:  info.Dc,
			Rid: info.Rid,
			Uid: info.Uid,
		}.BuildKey())
		trackFields := redis.HGetAll(path)

		var stream *media.Stream
		for key, value := range trackFields {
			if strings.HasPrefix(key, "track/") {
				stream, err = media.UnmarshalTracks(key, value)
				if err != nil {
					log.Errorf("%v", err)
				}
				log.Debugf("msid => %s, tracks => %v\n", stream.Id, stream.Tracks)
			}
		}

		log.Infof("Fields %v", fields)

		extraInfo := pb.UserMetadata{}
		if infoStr, ok := fields["info"]; ok {
			if err := json.Unmarshal([]byte(infoStr), &extraInfo); err != nil {
				log.Errorf("Unmarshal pub extra info %v", err)
				extraInfo = pb.UserMetadata{} // Needed?
			}
		}
		pubs = append(pubs, &pb.Pub{
			Minfo:  info,
			Uinfo:  &extraInfo,
			Stream: stream,
		})
	}

	log.Infof("getPubs: pubs=%v", pubs)
	return &pb.GetPubsReply{
		Pubs: pubs,
	}, nil
}

func (s *server) Join(ctx context.Context, in *pb.JoinRequest) (*pb.JoinReply, error) {
	// func clientJoin(data proto.JoinMsg) (interface{}, *nprotoo.Error) {
	ukey := pb.UserInfo{
		Dc:  dc,
		Rid: in.Rid,
		Uid: in.Uid,
	}.BuildKey()
	log.Infof("clientJoin: set %s => %v", ukey, in.Metadata)
	err := redis.HSetTTL(ukey, "info", in.Metadata, redisLongKeyTTL)
	if err != nil {
		log.Errorf("redis.HSetTTL err = %v", err)
	}
	log.Infof("Broadcast: peer-join = %v", in)
	broadcaster.Say(proto.IslbClientOnJoin, in)
	return &pb.JoinReply{}, nil
}

func (s *server) Leave(ctx context.Context, in *pb.LeaveRequest) (*pb.LeaveReply, error) {
	ukey := pb.UserInfo{
		Dc:  dc,
		Rid: in.Rid,
		Uid: in.Uid,
	}.BuildKey()
	log.Infof("clientLeave: remove key => %s", ukey)
	err := redis.Del(ukey)
	if err != nil {
		log.Errorf("redis.Del err = %v", err)
	}
	log.Infof("Broadcast peer-leave = %v", in)

	//make broadcast leave msg after remove stream msg, for ion block bug
	time.Sleep(500 * time.Millisecond)
	broadcaster.Say(proto.IslbClientOnLeave, data)
	return &pb.LeaveReply{}, nil
}

// func getMediaInfo(data proto.MediaInfo) (*media.Stream, *nprotoo.Error) {

func (s *server) GetStream(ctx context.Context, in *pb.GetStreamRequest) (*pb.GetStreamReply, error) {
	// Ensure DC
	in.Minfo.Dc = dc

	mkey := in.Minfo.BuildKey()
	log.Infof("getMediaInfo key=%s", mkey)

	if keys := redis.Keys(mkey + "*"); len(keys) > 0 {
		key := keys[0]
		log.Infof("Got: key => %s", key)
		fields := redis.HGetAll(key)

		// There should only be a single entry per stream
		for key, value := range fields {
			if strings.HasPrefix(key, "track/") {
				stream, err := media.UnmarshalTracks(key, value)
				if err != nil {
					log.Errorf("%v", err)
					return nil, fmt.Errorf("stream not found")
				}
				log.Infof("getMediaInfo: resp=%v", stream)
				return &pb.GetStreamReply{Stream: stream}, nil
			}
		}
	}

	return nil, fmt.Errorf("stream not found")
}

// func relay(data map[string]interface{}) (interface{}, *nprotoo.Error) {
// 	rid := util.Val(data, "rid")
// 	mid := util.Val(data, "mid")
// 	from := util.Val(data, "from")

// 	key := proto.GetPubNodePath(rid, mid)
// 	info := redis.HGetAll(key)
// 	for ip := range info {
// 		method := util.Map("method", proto.IslbRelay, "sid", from, "mid", mid)
// 		log.Infof("amqp.RpcCall ip=%s, method=%v", ip, method)
// 		//amqp.RpcCall(ip, method, "")
// 	}
// 	return struct{}{}, nil
// }

// func unRelay(data map[string]interface{}) (interface{}, *nprotoo.Error) {
// 	rid := util.Val(data, "rid")
// 	mid := util.Val(data, "mid")
// 	from := util.Val(data, "from")

// 	key := proto.GetPubNodePath(rid, mid)
// 	info := redis.HGetAll(key)
// 	for ip := range info {
// 		method := util.Map("method", proto.IslbUnrelay, "mid", mid, "sid", from)
// 		log.Infof("amqp.RpcCall ip=%s, method=%v", ip, method)
// 		//amqp.RpcCall(ip, method, "")
// 	}
// 	// time.Sleep(time.Millisecond * 10)
// 	resp := util.Map("mid", mid, "sid", from)
// 	log.Infof("unRelay: resp=%v", resp)
// 	return resp, nil
// }

func broadcast(data proto.BroadcastMsg) (interface{}, *nprotoo.Error) {
	log.Infof("broadcaster.Say msg=%v", data)
	broadcaster.Say(proto.IslbOnBroadcast, data)
	return struct{}{}, nil
}

// func handleRequest(rpcID string) {
// 	log.Infof("handleRequest: rpcID => [%v]", rpcID)

// 	protoo.OnRequest(rpcID, func(request nprotoo.Request, accept nprotoo.RespondFunc, reject nprotoo.RejectFunc) {
// 		go func(request nprotoo.Request, accept nprotoo.RespondFunc, reject nprotoo.RejectFunc) {
// 			method := request.Method
// 			msg := request.Data
// 			log.Infof("method => %s", method)

// 			var result interface{}
// 			err := util.NewNpError(400, fmt.Sprintf("Unkown method [%s]", method))

// 			switch method {
// 			case proto.IslbFindService:
// 				var msgData proto.FindServiceParams
// 				if err = msg.Unmarshal(&msgData); err == nil {
// 					result, err = findServiceNode(msgData)
// 				}
// 			case proto.IslbOnStreamAdd:
// 				var msgData proto.StreamAddMsg
// 				if err = msg.Unmarshal(&msgData); err == nil {
// 					result, err = streamAdd(msgData)
// 				}
// 			case proto.IslbOnStreamRemove:
// 				var msgData proto.StreamRemoveMsg
// 				if err = msg.Unmarshal(&msgData); err == nil {
// 					result, err = streamRemove(msgData)
// 				}
// 			case proto.IslbGetPubs:
// 				var msgData proto.RoomInfo
// 				if err = msg.Unmarshal(&msgData); err == nil {
// 					result, err = getPubs(msgData)
// 				}
// 			case proto.IslbClientOnJoin:
// 				var msgData proto.JoinMsg
// 				if err = msg.Unmarshal(&msgData); err == nil {
// 					result, err = clientJoin(msgData)
// 				}
// 			case proto.IslbClientOnLeave:
// 				var msgData proto.RoomInfo
// 				if err = msg.Unmarshal(&msgData); err == nil {
// 					result, err = clientLeave(msgData)
// 				}
// 			case proto.IslbGetMediaInfo:
// 				var msgData proto.MediaInfo
// 				if err = msg.Unmarshal(&msgData); err == nil {
// 					result, err = getMediaInfo(msgData)
// 				}
// 			// case proto.IslbRelay:
// 			// 	result, err = relay(data)
// 			// case proto.IslbUnrelay:
// 			// 	result, err = unRelay(data)
// 			case proto.IslbOnBroadcast:
// 				var msgData proto.BroadcastMsg
// 				if err = msg.Unmarshal(&msgData); err == nil {
// 					result, err = broadcast(msgData)
// 				}
// 			}

// 			if err != nil {
// 				reject(err.Code, err.Reason)
// 			} else {
// 				accept(result)
// 			}
// 		}(request, accept, reject)
// 	})
// }
