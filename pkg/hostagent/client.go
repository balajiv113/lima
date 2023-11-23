package hostagent

import (
	"context"
	"github.com/lima-vm/lima/pkg/limagrpc"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
	"net"
)

type GuestAgentClient struct {
	cli limagrpc.GuestServiceClient
}

func NewGuestAgentClient(dialFn func(ctx context.Context) (net.Conn, error)) (*GuestAgentClient, error) {
	var opts []grpc.DialOption

	opts = append(opts, grpc.WithContextDialer(func(ctx context.Context, target string) (net.Conn, error) {
		return dialFn(ctx)
	}))
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	clientConn, err := grpc.Dial("", opts...)
	if err != nil {
		return nil, err
	}
	client := limagrpc.NewGuestServiceClient(clientConn)
	return &GuestAgentClient{
		cli: client,
	}, nil
}

func (c *GuestAgentClient) Info(ctx context.Context) (*limagrpc.InfoResponse, error) {
	return c.cli.Info(ctx, &emptypb.Empty{})
}

func (c *GuestAgentClient) Events(ctx context.Context, eventCb func(response *limagrpc.EventResponse)) error {
	events, err := c.cli.Events(ctx, &emptypb.Empty{})
	if err != nil {
		return err
	}

	for {
		recv, err2 := events.Recv()
		if err2 != nil {
			return err2
		}
		eventCb(recv)
	}
}

func (c *GuestAgentClient) Inotify(ctx context.Context, inotifyCh chan *limagrpc.InotifyResponse) error {
	inotify, err := c.cli.Inotify(ctx)
	if err != nil {
		return err
	}
	go func() {
		for inotifyRes := range inotifyCh {
			err := inotify.Send(inotifyRes)
			if err != nil {
				logrus.WithError(err).Warn("failed to send inotify", err)
			}
		}
	}()
	return nil
}
