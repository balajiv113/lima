package limagrpc

import (
	"net"
	"strconv"
)

var IPv4loopback1 = net.IPv4(127, 0, 0, 1)

func (x *Port) HostString() string {
	return net.JoinHostPort(x.IP, strconv.Itoa(int(x.Port)))
}
