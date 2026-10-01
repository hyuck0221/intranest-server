# IntraNest 서버

팀의 공유 링크, 파일, 대화를 인트라넷 안의 PC에 보관합니다. IntraNest 서버는 직접 소유한 컴퓨터에서 동작하며, 콘텐츠를 외부 호스팅 서비스로 전송하지 않습니다.

**현재 버전:** `0.1.0`  
**언어:** [English](README.md)

## 주요 기능

- 공유 링크, 채팅 기록, 업로드 파일을 서버 PC에 저장합니다.
- 확장 프로그램과 IntraNest 웹 관리자 페이지가 하나의 인트라넷 주소로 연결됩니다.
- 채팅은 기본 30일 보관 후 삭제됩니다. 관리자 페이지의 설정에서 기간을 바꿀 수 있습니다.
- Docker Compose와 Windows/macOS/Linux용 단독 실행 파일을 제공합니다.
- 기본 파일 하나당 100 MiB, 전체 파일 10 GiB 제한으로 호스트 저장 공간을 보호합니다.

## Docker Compose로 시작하기

1. 서버로 사용할 컴퓨터에 Docker Desktop(Windows/macOS) 또는 Docker Engine과 Compose v2(Linux)를 설치합니다.
2. 이 저장소를 서버 컴퓨터에 내려받습니다.
3. 설정 스크립트를 실행합니다.

   ```sh
   ./scripts/setup.sh
   ```

   Windows에서는 WSL에서 실행하거나 Docker Desktop으로 compose 파일을 시작합니다.
4. 공개 IntraNest 웹사이트가 다른 origin에서 제공된다면 `.env`의 `INTRANEST_CORS_ORIGINS`에 정확한 origin을 추가하고 서버를 다시 시작합니다.
5. 서버 컴퓨터 방화벽에서 인트라넷에 한해 TCP `8080` 포트를 허용합니다.
6. 처음 시작하면 `data/access.token`에 접근 키가 생성됩니다. 신뢰할 수 있는 방법으로 팀에 전달합니다.
7. 웹 관리자 또는 확장 프로그램에서 `http://<서버-컴퓨터-주소>:8080`과 접근 키를 입력합니다.

연결 확인 API는 브라우저가 서버를 확인할 수 있도록 공개되어 있습니다. 링크, 파일, 채팅, 설정은 접근 키가 있어야 이용할 수 있습니다.

## Docker 없이 설치하기

서버 컴퓨터에 맞는 패키지를 [GitHub Releases](https://github.com/hyuck0221/intranest-server/releases)에서 내려받아 압축을 풀고 실행합니다. 처음 실행하면 `data` 폴더와 임의 접근 키가 생성됩니다.

- macOS/Linux: `./intranest`
- Windows: `intranest.exe`

기본 주소는 `0.0.0.0:8080`입니다. `data/access.token`에서 접근 키를 확인하고 컴퓨터 방화벽에서 인트라넷 포트를 허용합니다.

## 연결을 안전하게 유지하기

접근 키는 개인별 계정이 아니라 팀 공용 키입니다. 키를 가진 사용자는 공유 콘텐츠를 읽고 추가하고 삭제할 수 있으므로 안전한 방법으로 전달해야 합니다. 키를 바꾸려면 서버를 중지하고 `data/access.token`을 32자 이상의 새 임의 문자열로 교체한 뒤 다시 실행합니다.

채팅에 표시되는 이름은 작성자가 직접 입력하며 인증된 계정 정보가 아닙니다.

신뢰할 수 있는 인증서와 키를 `INTRANEST_TLS_CERT`, `INTRANEST_TLS_KEY`에 지정하면 HTTPS를 사용할 수 있습니다. HTTPS가 없으면 로컬 네트워크에서도 API와 접근 키가 암호화되지 않습니다. 신뢰 범위가 불분명한 네트워크에서는 사설 인증서 기관에서 발급한 신뢰 가능한 인증서를 사용하세요. `INTRANEST_CORS_ORIGINS`에는 웹 사이트의 정확한 origin을 지정하고 `*`는 사용하지 마세요.

Chrome은 웹 사이트가 로컬 네트워크 장치에 연결할 때 권한을 요청할 수 있습니다. 입력한 주소를 사용하려면 IntraNest 웹 사이트의 요청을 허용하세요. 확장 프로그램은 설정한 서버 origin에만 접근 권한을 요청합니다.

## API

JSON API는 `/api/v1`에서 제공합니다. 연결 확인, 접근 키, 링크, 채팅, 파일, 설정 API는 [API v1 문서](docs/api-v1.md)를 참고하세요.

## 데이터와 설정

기본 `./data` 폴더에 데이터베이스, 공유 파일, 접근 키를 저장합니다. 백업은 서버를 중지한 뒤 폴더 전체를 복사하세요. 주요 설정은 환경 변수로 변경할 수 있습니다.

| 환경 변수 | 기본값 | 설명 |
| --- | --- | --- |
| `INTRANEST_LISTEN_ADDR` | `0.0.0.0:8080` | 수신 인터페이스와 포트 |
| `INTRANEST_DATA_DIR` | `./data` | 데이터베이스, 파일, 키 폴더 |
| `INTRANEST_ACCESS_TOKEN` | 첫 실행 시 생성 | 팀 공용 접근 키 직접 지정 |
| `INTRANEST_CORS_ORIGINS` | 웹 origin 없음 | API 호출을 허용할 정확한 웹 origin 목록 |
| `INTRANEST_MAX_FILE_BYTES` | `104857600` | 파일 하나의 최대 크기(100 MiB, 1 MiB~2 GiB) |
| `INTRANEST_MAX_TOTAL_BYTES` | `10737418240` | 전체 공유 파일의 최대 크기(10 GiB, 최소 1 MiB) |
| `INTRANEST_TLS_CERT` / `INTRANEST_TLS_KEY` | 미설정 | HTTPS 인증서와 키 경로. 둘 다 설정해야 HTTPS가 켜집니다. |

채팅 보관 기간은 기본 30일이며 관리자 페이지에서 1~3,650일로 변경할 수 있습니다. 전체 파일 저장 공간은 기본 10 GiB이며 `INTRANEST_MAX_TOTAL_BYTES`로 바꿀 수 있습니다.

## 버전 배포

루트의 `version` 파일이 이 저장소의 버전을 관리합니다. `master`에 push할 때 현재 버전이 최신 GitHub Release보다 높으면 GitHub Release와 멀티 아키텍처 컨테이너를 배포합니다. 서버, 웹, 확장 프로그램 버전은 각각 따로 관리합니다.
