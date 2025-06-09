package system

import "net"

func GetFreePort() (int, error) {
	// Listen on TCP port 0 — means "assign a free port"
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer l.Close()

	// Extract the assigned port
	addr := l.Addr().(*net.TCPAddr)
	return addr.Port, nil
}
