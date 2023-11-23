package guestagent

import (
	"context"
	"github.com/lima-vm/lima/pkg/limagrpc"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"net"
)

func StartServer(lis net.Listener, guest *GuestServer) error {
	var opts []grpc.ServerOption
	server := grpc.NewServer(opts...)
	limagrpc.RegisterGuestServiceServer(server, guest)
	return server.Serve(lis)
}

type GuestServer struct {
	limagrpc.UnimplementedGuestServiceServer
	Agent Agent
}

func (s GuestServer) Info(ctx context.Context, _ *emptypb.Empty) (*limagrpc.InfoResponse, error) {
	return s.Agent.Info(ctx)
}

func (s GuestServer) Events(_ *emptypb.Empty, stream limagrpc.GuestService_EventsServer) error {
	responses := make(chan *limagrpc.EventResponse)
	go s.Agent.Events(context.Background(), responses)
	for response := range responses {
		err := stream.Send(response)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s GuestServer) Inotify(stream limagrpc.GuestService_InotifyServer) error {
	for {
		recv, err := stream.Recv()
		if err != nil {
			return err
		}
		s.Agent.HandleInotify(recv)
	}
}
