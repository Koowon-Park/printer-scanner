package main

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/k-sone/snmpgo"
)

// https://docs.bmc.com/docs/display/Configipedia/List+of+discoverable+printers

//https://github.com/librenms/librenms-mibs/blob/master/IANA-PRINTER-MIB
//https://github.com/librenms/librenms-mibs/blob/master/Printer-MIB
//https://github.com/librenms/librenms-mibs/blob/master/SAMSUNG-PRINTER-EXT-MIB

//172.26.7.5

//snmpget -v1 -cpublic 172.26.7.5 1.3.6.1.2.1.1.1.0
// snmpwalk -v1 -cpublic 172.26.7.5
// https://exchange.nagios.org/directory/Plugins/Hardware/Printers/check_snmp_printer/details
// https://github.com/coreyramirezgomez/Brother-Printers-Zabbix-Template/blob/master/check_snmp_printer.sh
// https://github.com/PetrKohut/SNMP-printer-library/blob/master/library/Kohut/SNMP/Printer.php
// http://docs.sharpsnmp.com/en/latest/tutorials/device-discovery.html
//https://www.webnms.com/telecom/help/developer_guide/discovery/discovery_process/disc_broadcast.html

// https://en.wikipedia.org/wiki/Service_Location_Protocol
// http://jslp.sourceforge.net/

// https://developer.apple.com/bonjour/printing-specification/bonjourprinting-1.2.pdf

// https://serverfault.com/questions/154650/find-printers-with-nmap

// https://developer.android.com/reference/android/net/nsd/NsdManager.html
// https://sharpsnmplib.codeplex.com/wikipage?title=SNMP%20Device%20Discovery&referringTitle=Documentation

// http://www.snmplink.org/cgi-bin/nd/m/*25[All]%20Draft/Printer-MIB-printmib-04.txt
///usr/libexec/cups/backend/snmp
//HOST-RESOURCES-MIB::hrDeviceType.1 = OID: HOST-RESOURCES-TYPES::hrDevicePrinter
//HOST-RESOURCES-MIB::hrDeviceDescr.1 = STRING: HP LaserJet 4000 Series

////// PRINTER MIB:  snmpwalk -v1 -c public 172.26.7.5 1.3.6.1.2.1.43

////https://sourceforge.net/projects/mpsbox/?source=directory
// https://github.com/k-sone/snmpgo/blob/master/examples/snmpgobulkwalk.go

// https://github.com/apple/cups/blob/master/backend/backend-private.h
// http://oid-info.com/get/1.3.6.1.2.1.43
// http://www.ietf.org/rfc/rfc1759.txt
// SNMP 조회에 사용할 공통 OID 목록 (CUPS 및 프린터 일반 정보)
var CUPS_OID = []string{
	"1.3.6.1.2.1.1.1.0",         // sysDescr: 시스템 설명
	"1.3.6.1.2.1.1.2.0",         // sysObjectID: 시스템 OID
	"1.3.6.1.2.1.1.5.0",         // sysName: 시스템 이름
	"1.3.6.1.2.1.43.5.1.1.17",   // 프린터 시리얼번호
	"1.3.6.1.2.1.43.5.1.1.16",   // 프린터 이름
	"1.3.6.1.2.1.43.10",         // 프린터 Marker 정보
	"1.3.6.1.2.1.43.11.1.1.6.1", // Marker Supplies 설명
	"1.3.6.1.2.1.43.11.1.1.8.1", // Supplies 최대 용량
	"1.3.6.1.2.1.43.11.1.1.9.1", // Supplies 잔여량
}

// http://www.oidview.com/mibs/2590/MC2350-MIB.html
var MINOLTA_OID = []string{
	// mltSysDuplexCount 1.3.6.1.4.1.2590.1.1.1.5.7.2.1.3
	// mltSysTotalCount 1.3.6.1.4.1.2590.1.1.1.5.7.2.1.1
	"1.3.6.1.4.1.2590.1.1.1.5.7.2",        // Minolta mltSysSystemCounter
	"1.3.6.1.4.1.18334.1.1.1.5.7.2.2.1.5", // Minolta MIB, kmSysPrintFunctionCounterTable
}

// http://www.oidview.com/mibs/11/IJXXXX-MIB.html
var HP_IJXXXX_OID = []string{
	// HP 프린터용 페이지 카운터 등
	// total-mono-page-count 1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.6
	// total-color-page-count 1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.7
	// duplex-page-count 1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.22
	"1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2",
}

// http://www.oidview.com/mibs/2435/BROTHER-MIB.html
var BROTHER_OID = []string{
	// Brother 프린터 정보
	"1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5", // Brother printerinfomation
}

// https://github.com/librenms/librenms-mibs/blob/master/SAMSUNG-PRINTER-EXT-MIB
var SAMSUNG_OID = []string{
	// Samsung 프린터용 OID
	"1.3.6.1.4.1.236.11.5.11.55.2.3.17",
	"1.3.6.1.4.1.236.11.5.11.55.2.3.20",
}
// 추가 OID 집합 목록 (비동기로 병렬 조회)
var EXTRA_OIDS = [][]string{
	MINOLTA_OID,
	HP_IJXXXX_OID,
	BROTHER_OID,
	SAMSUNG_OID,
}
// OID와 실제 속성명 매핑
var OID2PROP = map[string]string{
	"1.3.6.1.2.1.1.1.0": "sysDescr",
	"1.3.6.1.2.1.1.2.0": "sysObjectID",
	"1.3.6.1.2.1.1.5.0": "sysName",

	"1.3.6.1.2.1.43.5.1.1.17.1": "prtGeneralSerialNumber",
	"1.3.6.1.2.1.43.5.1.1.16.1": "prtGeneralPrinterName",

	"1.3.6.1.2.1.43.10.2.1.1.1.1":  "prtMarkerIndex",
	"1.3.6.1.2.1.43.10.2.1.2.1.1":  "prtMarkerMarkTech",
	"1.3.6.1.2.1.43.10.2.1.3.1.1":  "prtMarkerCounterUnit",
	"1.3.6.1.2.1.43.10.2.1.4.1.1":  "prtMarkerLifeCount",
	"1.3.6.1.2.1.43.10.2.1.5.1.1":  "prtMarkerPowerOnCount",
	"1.3.6.1.2.1.43.10.2.1.15.1.1": "prtMarkerStatus",

	"1.3.6.1.2.1.43.11.1.1.1": "prtMarkerSuppliesIndex",

	// hp
	"1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.5.0":  "total-engine-page-count",
	"1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.6.0":  "total-mono-page-count",
	"1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.7.0":  "total-color-page-count",
	"1.3.6.1.4.1.11.2.3.9.4.2.1.4.1.2.22.0": "duplex-page-count",

	// brother
	"1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.1":  "brInfoSerialNumber",
	"1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.10": "brInfoCounter",
	"1.3.6.1.4.1.2435.2.3.9.4.2.1.5.5.17": "brInfoDeviceRomVersion",

	// Minolta
	"1.3.6.1.4.1.18334.1.1.1.5.7.2.2.1.5.1.1": "copy-counter-black",
	"1.3.6.1.4.1.18334.1.1.1.5.7.2.2.1.5.2.1": "copy-counter-color",
	"1.3.6.1.4.1.18334.1.1.1.5.7.2.2.1.5.1.2": "print-counter-black",
	"1.3.6.1.4.1.18334.1.1.1.5.7.2.2.1.5.2.2": "print-counter-color",
}

// 지정된 IP와 OID 목록을 이용해 SNMP로 정보를 조회
// ip: 조회할 프린터의 IP 주소
// oidsToScan: 조회할 OID 목록
// 성공 시 VarBinds 반환, 실패 시 에러 반환
func snmpScanOIDS(ip net.IP, oidsToScan []string) (varBinds snmpgo.VarBinds, err error) {
	ip = ip.To4()
	if ip == nil {
		return nil, errors.New("Scan IP failed - IP is not specified!") // IPv4 주소가 아님
	}
	// SNMP 클라이언트 생성
	snmp, err := snmpgo.NewSNMP(snmpgo.SNMPArguments{
		Version:   snmpgo.V2c,                   // SNMP v2c 사용
		Address:   fmt.Sprintf("%v:161", ip),    // SNMP 기본 포트 161
		Retries:   1,                            // 재시도 횟수
		Community: "public",                     // 커뮤니티 문자열
	})
	if err != nil {
		// log.Printf("Failed to allocate SNMP Request: %v\n", err)
		return nil, err
	}
	// OID 구문 분석
	oids, err := snmpgo.NewOids(oidsToScan) /*Add commentMore actions*/

	if err != nil {
		// log.Printf("Failed to parse Oids: %v\n", err)
		return nil, err
	}
	// SNMP 연결 오픈
	if err = snmp.Open(); err != nil {
		// log.Printf("Failed to open connection to %v: %v\n", ip, err)
		return
	}
	defer snmp.Close()

	// Bulk Walk로 여러 OID를 한번에 조회
	// pdu, err := snmp.GetRequest(oids)
	var nonRepeaters = 0
	var maxRepetitions = 10
	pdu, err := snmp.GetBulkWalk(oids, nonRepeaters, maxRepetitions)
	if err != nil {
		return nil, err
	}
	return pdu.VarBinds(), nil
}

// 프린터 한 대(IP)에 대해 주요 OID 및 제조사별 추가 OID를 병렬로 SNMP 조회
func SnmpScan(ip net.IP) (varBinds snmpgo.VarBinds, err error) {
	// 공통 OID로 기본 정보 조회
	vBinds, err := snmpScanOIDS(ip, CUPS_OID)
	if err != nil {
		return nil, err
	}

	// 제조사별 OID를 비동기 조회, 결과를 vBinds에 합침
	var wg sync.WaitGroup
	for _, oids := range EXTRA_OIDS {
		wg.Add(1)
		go func(xip net.IP, xoidx []string) {
			defer wg.Done()
			vBinds2, _ := snmpScanOIDS(xip, xoidx)
			if vBinds2 != nil {
				vBinds = append(vBinds, vBinds2...)
			}
		}(ip, oids)
	}
	wg.Wait()
	return vBinds, nil
}

// SNMP 조회 결과를 사람이 읽을 수 있는 포맷으로 출력
// w: 출력 Writer (파일, 표준출력 등)
// ip: 프린터 IP
// vars: SNMP 조회 결과 바인딩 값들
func SnmpPrint(w io.Writer, ip net.IP, vars snmpgo.VarBinds) {
	fmt.Fprintf(w, "[%v]\n", ip)
	if vars != nil {
		for _, val := range vars {
			oidS := val.Oid.String()
			prop, ok := OID2PROP[oidS]
			if !ok {
				prop = oidS // 맵핑이 없으면 OID 자체 출력
			}
			fmt.Fprintf(w, "%s = %s\n", prop, val.Variable)
		}
	}
	fmt.Fprintf(w, "\n\n")
}

// JSON 출력을 위한 구조체
type JsonVar struct {
	Key   string // 속성명
	Value string // 값
}
type JsonVars struct {
	Ip   string            // 프린터 IP
	Data map[string]string // OID 또는 속성명-값 맵
}

// SNMP 결과를 JSON 구조체로 변환 (API 응답 등에 사용)
func Snmp2Json(ip net.IP, vars snmpgo.VarBinds) (ret JsonVars) {
	ret = JsonVars{
		Ip:   ip.String(),
		Data: make(map[string]string),
	}
	if vars != nil {
		for _, val := range vars {
			oidS := val.Oid.String()
			prop, ok := OID2PROP[oidS]
			if !ok {
				prop = oidS
			}
			ret.Data[prop] = val.Variable.String()
		}
	}
	return ret
}
