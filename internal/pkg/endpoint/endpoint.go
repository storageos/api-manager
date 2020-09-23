package endpoint

import (
	"net"
	"strconv"
)

// SplitAddressPort splits a network address of the form "host:port",
// "host%zone:port", "[host]:port" or "[host%zone]:port" into host or host%zone
// and port.
//
// Unlike net.SplitHostPort, the Port is returned as an int.
func SplitAddressPort(endpoint string) (string, int, error) {
	address, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return "", 0, err
	}
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return "", 0, err
	}
	return address, portNum, nil
}
