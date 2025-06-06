package main

import (
	"bytes"         // 바이트 버퍼를 사용하기 위한 패키지 (HTTP 요청 시 사용)
	"encoding/json" // JSON 인코딩/디코딩을 위한 패키지
	"flag"          // 커맨드라인 플래그 파싱을 위한 패키지
	"log"           // 로그 출력을 위한 패키지
	"net"           // 네트워크 관련 함수(IP, Lookup 등)
	"net/http"      // HTTP 통신을 위한 패키지
	"os"            // 운영체제 관련 함수 및 파일 입출력
	"sync"          // 고루틴 동기화(WaitGroup)를 위한 패키지
)

// 커맨드라인 플래그 정의
var doScan   = flag.Bool("scan", false, "Scan the complete local network to find Printer Devices.") // 네트워크 전체 검색 여부
var outFile  = flag.String("o", "", "The output file, where the scan results to be written.")        // 결과 출력 파일명
var url      = flag.String("post", "", "Optional URL. When specified the printer data is serialized to JSON and posted to that URL.") // 결과를 JSON으로 전송할 URL
var clientId = flag.String("clientId", "", "Option clientId, that is supplied when posting printer data to URL.") // 결과 전송 시 함께 보낼 clientId

// 프린터 정보와 클라이언트 ID를 담는 구조체
type PostData struct {
	ClientId string      // POST 시 함께 보낼 클라이언트 ID
	Printers []JsonVars  // 프린터 정보 목록 (JsonVars는 프린터별 정보 구조체)
}

// 프린터 정보를 JSON으로 직렬화해서 지정한 URL로 전송하는 함수
func postPrinterData(d PostData) {
	log.Printf("Publishing printer data to URL: %s\n", *url) // 전송 시작 로그

	// 구조체를 JSON 문자열로 변환
	jsonString, _ := json.Marshal(d)

	// HTTP POST 요청을 전송
	resp, err := http.Post(*url, "application/json", bytes.NewBuffer(jsonString))
	if err != nil { // 오류 발생 시 로그 출력 후 함수 종료
		log.Printf("Failed to publish printer data: %v\n", err)
		return
	}
	defer resp.Body.Close() // 함수 종료 시 응답 바디 닫기
	log.Printf("Publishing printer data to URL '%s' completed with code: %s\n", *url, resp.Status) // 결과 상태 로그
}

// 프로그램 진입점
func main() {
	flag.Parse() // 커맨드라인 플래그 파싱

	var ipList []net.IP      // 스캔할 IP 주소 목록
	out := os.Stdout         // 기본 출력은 터미널 콘솔

	// 결과 파일이 지정됐다면, 파일로 출력하도록 설정
	if outFile != nil && *outFile != "" {
		var err error
		out, err = os.Create(*outFile) // 파일 생성 시도
		if err != nil {                // 파일 생성 실패 시
			log.Printf("Cannot open file %v for writing, using console instead!\n", outFile)
			defer out.Close()
			out = os.Stdout // 콘솔로 대체
			*outFile = "console"
		}
	}

	// -scan 플래그가 지정된 경우, 네트워크상의 모든 IP 목록을 가져옴
	if *doScan {
		ipList, _ = NetGetNetworkIPs() // (구현은 다른 파일에 있음)
	}

	// 커맨드라인 인수로 직접 IP/도메인을 입력받은 경우 모두 추가
	for _, arg := range flag.Args() {
		ips, _ := net.LookupIP(arg) // 도메인/호스트명 → IP 변환
		for _, ip := range ips {
			ipList = append(ipList, ip)
		}
	}

	// 실제로 스캔할 IP가 있는 경우
	if ipList != nil {
		// POST를 위한 데이터 구조체 초기화
		json := PostData{
			ClientId: *clientId,
			Printers: []JsonVars{},
		}

		log.Printf("Scanning started ...\n")

		// 동시 IP 스캔을 위한 WaitGroup 생성
		var wg sync.WaitGroup
		for _, ip := range ipList {
			wg.Add(1) // 고루틴 개수 증가
			go func(xip net.IP) { // 각 IP마다 고루틴 실행
				defer wg.Done() // 고루틴 종료 시 카운트 감소

				vars, _ := SnmpScan(xip) // SNMP로 프린터 정보 스캔
				if vars != nil {         // 성공 시
					log.Printf("%16v -> OK\n", xip)
					json.Printers = append(json.Printers, Snmp2Json(xip, vars)) // JSON 데이터에 추가
					SnmpPrint(out, xip, vars) // 결과 파일/콘솔에 출력
				} else { // 실패 시
					log.Printf("%16v -> FAIL\n", xip)
				}
			}(ip)
		}
		wg.Wait() // 모든 고루틴이 끝날 때까지 대기

		// 결과를 외부 URL로 POST 전송하는 옵션이 있다면 실행
		if *url != "" {
			postPrinterData(json)
		}
		log.Printf("Scanning complete! Results are printed to %v\n", *outFile) // 완료 로그
	} else {
		log.Printf("No printers to scan, please provide a list of printers or -scan parameter!\n") // 스캔할 IP가 없을 때
	}
}
