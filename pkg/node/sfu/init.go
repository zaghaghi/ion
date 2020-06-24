package sfu

import (
	"net"

	"github.com/pion/ion/pkg/log"
	"google.golang.org/grpc"

	pb "github.com/pion/ion/pkg/proto/sfu"
)

type server struct {
	pb.UnimplementedSFUServer
}

// Init func
func Init(port string) {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Panicf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterSFUServer(s, &server{})
	if err := s.Serve(lis); err != nil {
		log.Panicf("failed to serve: %v", err)
	}

	checkRTC()
}

// checkRTC send `stream-remove` msg to islb when some pub has been cleaned
func checkRTC() {
	log.Infof("SFU.checkRTC start")
	// go func() {
	// 	for mid := range rtc.CleanChannel {
	// 		broadcaster.Say(proto.SFUStreamRemove, util.Map("mid", mid))
	// 	}
	// }()
}
