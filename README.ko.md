<p align="center"><img src="docs/assets/intranest-mark.png" width="112" alt="IntraNest 로고"></p>

# IntraNest 서버

팀의 링크, 파일, 대화를 직접 관리하는 PC에 보관합니다. 서버는 인트라넷 안에서 동작하며 공유 자료를 자체 데이터 볼륨에 저장합니다.

**현재 버전:** `0.2.0`

**언어:** [English](README.md)

## 서버 PC에 설치하기

Docker Desktop(Windows/macOS) 또는 Docker Engine(Linux)을 설치하고 실행한 뒤 다음 명령을 실행하세요.

```sh
curl -fsSL https://github.com/hyuck0221/intranest-server/releases/latest/download/install.sh | sh
```

공식 컨테이너를 내려받아 실행하므로 저장소를 clone하거나 fork할 필요가 없습니다. Windows에서는 Docker Desktop을 실행하고 WSL 또는 Git Bash에서 명령을 실행하세요. 서버는 컴퓨터의 네트워크 인터페이스 전체에서 기본 포트 `43187`로 연결을 받습니다.

관리자 페이지나 Chrome 확장 프로그램에는 `192.168.1.40`처럼 서버 PC의 IP 주소나 호스트 이름만 입력하세요. 포트 `43187`은 자동으로 사용합니다. 호스트 컴퓨터 방화벽에서 사용할 네트워크에 한해 TCP `43187` 포트를 허용합니다.

클라이언트는 HTTPS로 먼저 연결하고 실패하면 HTTP로 다시 시도합니다. 암호화 연결이 필요하면 서버에 신뢰할 수 있는 인증서를 설정하세요.

HTTPS로 호스팅된 관리자 페이지에서 채팅을 사용하려면 IntraNest 서버도 HTTPS를 설정해야 합니다. 인증서와 키를 호스트 PC의 한 폴더에 넣고, 폴더와 컨테이너 안의 파일 이름을 지정해 설치하세요.

```sh
curl -fsSL https://github.com/hyuck0221/intranest-server/releases/latest/download/install.sh | INTRANEST_CERT_DIR="$PWD/certs" INTRANEST_TLS_CERT=/certs/fullchain.pem INTRANEST_TLS_KEY=/certs/privkey.pem sh
```

클라이언트 기기에서 신뢰하는 인증서를 사용하고 서버 주소와 인증서 이름이 일치해야 합니다. 설치 스크립트는 파일을 Docker 볼륨에 복사하고 서버에는 읽기 전용으로 연결합니다.

## 팀에서 공유할 수 있는 자료

- 팀에서 자주 쓰는 사이트 링크
- 서버 PC에 저장되는 파일
- 실시간 채팅과 기본 30일 보관 기록

채팅 보관 기간은 관리자 페이지 → 설정에서 바꿀 수 있습니다. 파일은 기본 파일당 100 MiB, 전체 10 GiB까지 업로드할 수 있습니다.

## 네트워크 접근

IntraNest에는 로그인이나 접근 키가 없습니다. 서버에 연결할 수 있는 사람은 링크, 파일, 채팅을 읽고 추가하고 삭제할 수 있습니다. 서버는 모든 웹 origin의 브라우저 요청을 허용하고 기본적으로 모든 네트워크 인터페이스에서 연결을 받습니다. 신뢰할 수 있는 네트워크에서 실행하고 컴퓨터 방화벽으로 포트를 제한하며 공용 인터넷에 노출하지 마세요.

채팅 표시 이름은 작성자가 직접 입력하며 인증된 계정이 아닙니다. 기본 HTTP 연결은 데이터를 암호화하지 않습니다. 암호화가 필요하면 신뢰할 수 있는 네트워크 endpoint에서 HTTPS를 설정하세요.

## 서버 관리

- 상태 확인: `docker ps --filter name=intranest-server`
- 로그 보기: `docker logs -f intranest-server`
- 중지: `docker stop intranest-server`
- 다시 시작: `docker start intranest-server`

설치 스크립트는 데이터베이스와 업로드 파일을 Docker 볼륨 `intranest-data`에 보관합니다. 컨테이너를 지워도 이 볼륨은 유지됩니다.

## 설치 설정

설치 명령을 실행하기 전에 다음 환경 변수를 선택적으로 지정할 수 있습니다.

| 변수 | 기본값 | 설명 |
| --- | --- | --- |
| `INTRANEST_VERSION` | `latest` | 설치할 컨테이너 버전 |
| `INTRANEST_BIND_ADDRESS` | `0.0.0.0` | 연결을 받을 호스트 인터페이스 |
| `INTRANEST_DATA_VOLUME` | `intranest-data` | 데이터베이스와 파일을 보관할 Docker 볼륨 |
| `INTRANEST_CERT_DIR` | — | HTTPS 사용 시 인증서 파일이 들어 있는 호스트 폴더 |
| `INTRANEST_CERT_VOLUME` | `<container>-certs` | 서버의 읽기 전용 인증서 파일이 들어 있는 Docker 볼륨 |
| `INTRANEST_TLS_CERT` | — | 컨테이너 내부 인증서 경로(예: `/certs/fullchain.pem`) |
| `INTRANEST_TLS_KEY` | — | 컨테이너 내부 개인 키 경로(예: `/certs/privkey.pem`) |
| `INTRANEST_MAX_FILE_BYTES` | `104857600` | 파일 하나의 최대 업로드 크기 |
| `INTRANEST_MAX_TOTAL_BYTES` | `10737418240` | 전체 파일 저장 한도 |

Linux, macOS, Windows용 단독 실행 파일도 [GitHub Releases](https://github.com/hyuck0221/intranest-server/releases)에서 받을 수 있습니다. 개발자용 API는 [API v2 가이드](docs/api-v2.md)를 참고하세요.

## 버전 배포

루트의 `version` 파일이 이 프로젝트 버전을 관리합니다. `master`에 push할 때 현재 버전이 최신 릴리스보다 높으면 GitHub Release, 설치 스크립트, 운영체제별 패키지, 멀티 아키텍처 컨테이너를 배포합니다. 서버, 웹, 확장 프로그램 버전은 각각 따로 관리합니다.
