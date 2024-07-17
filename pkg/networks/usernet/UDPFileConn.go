package usernet

import (
	"errors"
	"net"
	"syscall"
	"time"
)

type UDPFileConn struct {
	net.Conn
}

func (conn *UDPFileConn) Read(b []byte) (n int, err error) {
	// Check if the connection has been closed
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		if opErr, ok := err.(*net.OpError); ok && opErr.Err.Error() == "use of closed network connection" {
			return 0, errors.New("UDPFileConn connection closed")
		}
	}
	return conn.Conn.Read(b)
}

func (conn *UDPFileConn) Write(b []byte) (n int, err error) {
	write, err := conn.Conn.Write(b)
	if errors.Is(err, syscall.ENOBUFS) {
		return conn.Write(b)
	}
	return write, err
}
