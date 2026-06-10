package utils

import "net"

// ResolveLocalIPv4 解析本机 IPv4（优先可路由的出站 IP，其次非回环地址，最后 127.0.0.1）
func ResolveLocalIPv4() string {
	// 优先：通过 UDP Dial 推断出站 IP（不建立实际连接）
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		if la, ok := conn.LocalAddr().(*net.UDPAddr); ok && la.IP != nil {
			return la.IP.String()
		}
	}
	// 回退：枚举网卡，选取第一个非回环的 IPv4
	addrs, err := net.InterfaceAddrs()
	if err == nil {
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	// 兜底
	return "127.0.0.1"
}
