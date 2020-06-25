package islb

import (
	"net"
	"time"

	"github.com/pion/ion/pkg/db"
	"github.com/pion/ion/pkg/discovery"
	"github.com/pion/ion/pkg/log"
	"google.golang.org/grpc"

	pb "github.com/pion/ion/pkg/proto/islb"
)

const (
	redisLongKeyTTL = 24 * time.Hour
)

type server struct {
	pb.UnimplementedISLBServer
}

var (
	dc = "default"
	//nolint:unused
	nid      = "islb-unkown-node-id"
	redis    *db.Redis
	services map[string]discovery.Node
)

// Init func
func Init(dcID, nodeID, port string, redisCfg db.Config, etcd []string, natsURL string) {
	dc = dcID
	nid = nodeID
	redis = db.NewRedis(redisCfg)
	services = make(map[string]discovery.Node)

	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Panicf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterISLBServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Panicf("failed to serve: %v", err)
	}

	WatchAllStreams()
}
