package hostagent

import (
	"context"
	"github.com/lima-vm/lima/pkg/limagrpc"
	"net"

	"github.com/lima-vm/lima/pkg/limayaml"
	"github.com/lima-vm/sshocker/pkg/ssh"
	"github.com/sirupsen/logrus"
)

type portForwarder struct {
	sshConfig   *ssh.SSHConfig
	sshHostPort int
	rules       []limayaml.PortForward
	vmType      limayaml.VMType
}

const sshGuestPort = 22

func newPortForwarder(sshConfig *ssh.SSHConfig, sshHostPort int, rules []limayaml.PortForward, vmType limayaml.VMType) *portForwarder {
	return &portForwarder{
		sshConfig:   sshConfig,
		sshHostPort: sshHostPort,
		rules:       rules,
		vmType:      vmType,
	}
}

func hostAddress(rule limayaml.PortForward, guest *limagrpc.Port) string {
	if rule.HostSocket != "" {
		return rule.HostSocket
	}
	host := &limagrpc.Port{IP: rule.HostIP.String()}
	if guest.Port == 0 {
		// guest is a socket
		host.Port = int32(rule.HostPort)
	} else {
		host.Port = guest.Port + int32(rule.HostPortRange[0]-rule.GuestPortRange[0])
	}
	return host.HostString()
}

func (pf *portForwarder) forwardingAddresses(guest *limagrpc.Port, localUnixIP net.IP) (string, string) {
	guestIp := net.ParseIP(guest.IP)
	if pf.vmType == limayaml.WSL2 {
		guestIp = localUnixIP
		host := &limagrpc.Port{
			IP:   net.ParseIP("127.0.0.1").String(),
			Port: guest.Port,
		}
		return host.String(), guest.HostString()
	}
	for _, rule := range pf.rules {
		if rule.GuestSocket != "" {
			continue
		}
		if guest.Port < int32(rule.GuestPortRange[0]) || guest.Port > int32(rule.GuestPortRange[1]) {
			continue
		}
		switch {
		case guestIp.IsUnspecified():
		case guestIp.Equal(rule.GuestIP):
		case guestIp.Equal(net.IPv6loopback) && rule.GuestIP.Equal(limagrpc.IPv4loopback1):
		case rule.GuestIP.IsUnspecified() && !rule.GuestIPMustBeZero:
			// When GuestIPMustBeZero is true, then 0.0.0.0 must be an exact match, which is already
			// handled above by the guest.IP.IsUnspecified() condition.
		default:
			continue
		}
		if rule.Ignore {
			if guestIp.IsUnspecified() && !rule.GuestIP.IsUnspecified() {
				continue
			}
			break
		}
		return hostAddress(rule, guest), guest.HostString()
	}
	return "", guest.HostString()
}

func (pf *portForwarder) OnEvent(ctx context.Context, ev *limagrpc.EventResponse, instSSHAddress string) {
	localUnixIP := net.ParseIP(instSSHAddress)

	for _, f := range ev.LocalPortsRemoved {
		local, remote := pf.forwardingAddresses(f, localUnixIP)
		if local == "" {
			continue
		}
		logrus.Infof("Stopping forwarding TCP from %s to %s", remote, local)
		if err := forwardTCP(ctx, pf.sshConfig, pf.sshHostPort, local, remote, verbCancel); err != nil {
			logrus.WithError(err).Warnf("failed to stop forwarding tcp port %d", f.Port)
		}
	}
	for _, f := range ev.LocalPortsAdded {
		local, remote := pf.forwardingAddresses(f, localUnixIP)
		if local == "" {
			logrus.Infof("Not forwarding TCP %s", remote)
			continue
		}
		logrus.Infof("Forwarding TCP from %s to %s", remote, local)
		if err := forwardTCP(ctx, pf.sshConfig, pf.sshHostPort, local, remote, verbForward); err != nil {
			logrus.WithError(err).Warnf("failed to set up forwarding tcp port %d (negligible if already forwarded)", f.Port)
		}
	}
}
