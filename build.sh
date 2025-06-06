#!/bin/sh
# 항상 현재 스크립트가 위치한 디렉터리에서 실행
cd `dirname $0`

# CGO(Go의 C언어 바인딩 기능) 비활성화로 완전 정적 빌드 설정
CGO_ENABLED="0"
export CGO_ENABLED

# Go Modules를 사용하므로 GOPATH 환경변수 제거
unset GOPATH

# 첫 번째 인자에 따라 분기
case "$1" in
    "build.linux")
        # 리눅스용 바이너리 빌드: 
        # -a : 전체 패키지 재컴파일
        # -ldflags: 빌드 시 추가 플래그 
        #   --s : 심볼 정보 제거(바이너리 크기 감소)
        #   -extldflags '-static' : 정적 링크
        #   -X main.Version=git:$CI_BUILD_REF : 버전 정보 삽입
        # -o : 출력 파일명 지정
        # ./... : 모든 하위 디렉터리 포함
        go build -a -ldflags "--s -extldflags '-static' -X main.Version=git:$CI_BUILD_REF" -o "printer-scanner$SUFFIX" ./...
        ;;
    "build.mac")
        # macOS용 빌드를 위한 환경 변수 설정
        GOOS="darwin"          # 빌드 대상 OS
        GOARCH="amd64"         # 빌드 대상 아키텍처
        SUFFIX=".$GOOS-$GOARCH" # 출력 파일명에 접미사 추가
        export GOOS GOARCH SUFFIX
        # 리눅스 빌드 명령 재사용 (환경변수로 OS/아키텍처 지정)
        $0 build.linux
        ;;
    "build.win")
        # Windows용 빌드를 위한 환경 변수 설정
        GOOS="windows"
        GOARCH="amd64"
        SUFFIX=".$GOOS-$GOARCH.exe" # 윈도우 확장자 및 접미사 추가
        export GOOS GOARCH SUFFIX
        # 리눅스 빌드 명령 재사용 (환경변수로 OS/아키텍처 지정)
        $0 build.linux
        ;;
    "build")
        # 리눅스, 맥, 윈도우용 빌드를 한 번에 실행
        $0 build.linux
        $0 build.mac
        $0 build.win
        ;;
    "shell")
        # 도커 컨테이너(Go 1.14 이미지) 내에서 bash 쉘 실행 (빌드 환경 제공)
        shift
        docker run -it --rm --name printer-scanner-builder -v `pwd`:/go golang:1.14 /bin/bash
        ;;
    *)
        # 인자가 없거나 알 수 없는 경우: 
        # 도커 컨테이너에서 이 build.sh를 실행해 3종 빌드 수행
        docker run -it --rm --name printer-scanner-builder -v `pwd`:/go golang:1.14 /bin/sh -c "/go/build.sh build"
esac
