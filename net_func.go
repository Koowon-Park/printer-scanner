package main

import (
	"log"
	"net"
)

// dupIP 함수는 입력받은 IP 주소를 복사하여 새로운 net.IP 객체로 반환합니다.
// 원본 IP를 직접 수정하지 않기 위해 사용됩니다.
func dupIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	return dup
}

// inc 함수는 입력받은 IP 주소를 1 증가시킵니다.
// 네트워크 주소 범위에서 다음 IP로 이동할 때 사용됩니다.
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		// 바이트가 오버플로우하지 않았다면 반복문 종료
		if ip[j] > 0 {
			break
		}
	}
}

// NetGetNetworkIPs 함수는 시스템의 네트워크 인터페이스를 순회하며
// 사용 가능한 IPv4 네트워크의 모든 IP 주소를 리스트로 반환합니다.
// 에러가 발생하면 nil과 에러를 반환합니다.
func NetGetNetworkIPs() (ip []net.IP, err error) {
	var ipList []net.IP

	// 시스템의 모든 네트워크 인터페이스 정보 조회
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	// 모든 인터페이스를 순회
	for _, iface := range ifaces {
		// 인터페이스가 활성화되어 있고, 브로드캐스트가 가능한 경우만 처리
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagBroadcast != 0 {
			addrs, err := iface.Addrs()
			if err != nil {
				log.Printf("Error reading interface address %v : %v", iface, err)
			} else {
				// 인터페이스의 모든 주소를 순회
				for _, addr := range addrs {
					ip, ipnet, err := net.ParseCIDR(addr.String())
					if err != nil {
						log.Printf("Error parsing CIDR %s : %v", addr.String(), err)
					} else {
						ip = ip.To4()  // IPv4 주소만 처리
						if ip != nil { // IPv6는 무시
							log.Printf("Scanning network interface '%v' with CIDR = %v, IP = %v\n", iface.Name, addr.String(), ip)
							// 해당 네트워크 대역의 모든 IP를 순회
							for xip := ip.Mask(ipnet.Mask); ipnet.Contains(xip); inc(xip) {
								ipList = append(ipList, dupIP(xip))
							}
						}
					}
				}
			}
		}
	}

	return ipList, nil
}
