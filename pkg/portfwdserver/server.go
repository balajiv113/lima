package portfwdserver

import (
	"bufio"
	"errors"
	"github.com/lima-vm/lima/pkg/guestagent/api"
	"github.com/sirupsen/logrus"
	"io"
	"net"
)

type TunnelServer struct {
	Conns map[string]net.Conn
}

func NewTunnelServer() *TunnelServer {
	return &TunnelServer{
		Conns: make(map[string]net.Conn),
	}
}

func (s *TunnelServer) Start(stream api.GuestService_TunnelServer) error {
	logrus.Println("start tunnel")
	for {
		in, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			logrus.Println("start tunnel err", err)
			return err
		}

		logrus.Println("start tunnel", in.Id, s.Conns)
		_, ok := s.Conns[in.Id]
		if !ok {
			//Retry on connect failure
			for {
				conn, err := net.Dial(in.Protocol, in.GuestAddr)
				if err != nil {
					return err
				}
				reader := bufio.NewReader(conn)
				_, err = reader.Peek(1)
				if err != nil {
					if errors.Is(err, io.EOF) {
						continue
					}
					return err
				}

				s.Conns[in.Id] = conn
				writer := &GRPCServerWriter{id: in.Id, udpAddr: in.UdpTargetAddr, stream: stream}
				go func() {
					_, _ = io.Copy(writer, reader)
					delete(s.Conns, writer.id)
				}()
				break
			}
		}
		_, err = s.Conns[in.Id].Write(in.Data)
		if err != nil {
			return err
		}
	}
}

type GRPCServerWriter struct {
	id      string
	udpAddr string
	stream  api.GuestService_TunnelServer
}

var _ io.Writer = (*GRPCServerWriter)(nil)

func (g GRPCServerWriter) Write(p []byte) (n int, err error) {
	err = g.stream.Send(&api.TunnelMessage{Id: g.id, Data: p, UdpTargetAddr: g.udpAddr})
	logrus.Println("GRPC", len(p), err)
	return len(p), err
}
