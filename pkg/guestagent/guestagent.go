package guestagent

import (
	"context"
	"github.com/lima-vm/lima/pkg/limagrpc"
)

type Agent interface {
	Info(ctx context.Context) (*limagrpc.InfoResponse, error)
	Events(ctx context.Context, ch chan *limagrpc.EventResponse)
	HandleInotify(event *limagrpc.InotifyResponse)
}
