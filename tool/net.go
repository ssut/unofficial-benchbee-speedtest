package tool

import (
	"net"
	"strings"
)

func GetTcpAddrFromInterfaceName(name string) (*net.TCPAddr, error) {
	ief, err := net.InterfaceByName(name)
	if err != nil {
		return nil, err
	}

	addrs, err := ief.Addrs()
	if err != nil {
		return nil, err
	}

	// only IPv4
	var targetAddr net.Addr
	for _, addr := range addrs {
		if strings.Contains(addr.String(), ".") && !strings.Contains(addr.String(), ":") {
			targetAddr = addr
		}
	}

	tcpAddr := &net.TCPAddr{
		IP: targetAddr.(*net.IPNet).IP,
	}
	return tcpAddr, nil
}

func CreateDialerUsingInterfaceName(name string) (*net.Dialer, error) {
	tcpAddr, err := GetTcpAddrFromInterfaceName(name)
	if err != nil {
		return nil, err
	}
	dialer := &net.Dialer{

		LocalAddr: tcpAddr,
	}
	return dialer, nil
}

func CreateDialerUsingIPAddress(ipAddress string) (*net.Dialer, error) {
	tcpAddr := &net.TCPAddr{
		IP: net.ParseIP(ipAddress),
	}
	dialer := &net.Dialer{
		LocalAddr: tcpAddr,
	}
	return dialer, nil
}
