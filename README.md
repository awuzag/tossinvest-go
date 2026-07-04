# tossinvest-go

토스증권 OpenAPI를 사용하기 위한 비공식 Go SDK 및 CLI입니다.

> **비공식 / 개발 중**
>
> 이 프로젝트는 토스증권 또는 Toss Invest가 공식 제공, 보증, 승인, 유지보수하는 SDK가 아닙니다.
> 공개된 Toss Invest OpenAPI 문서를 바탕으로 awuzag에서 개발 중인 비공식 클라이언트입니다.
> 아직 개발 중이므로 SDK API, CLI 옵션, generated code 구조가 바뀔 수 있습니다.

## 공식 참고 링크

- 토스증권 Open API 소개: https://corp.tossinvest.com/ko/open-api
- 토스증권 Open API 가이드: https://developers.tossinvest.com/docs
- 공식 OpenAPI JSON: https://openapi.tossinvest.com/openapi-docs/latest/openapi.json

이 저장소는 공식 OpenAPI JSON을 자동으로 다운로드하지 않습니다. 검토한 snapshot을 `contracts/tossinvest/openapi.json`에 반영하고, 그 contract를 기준으로 generated code와 drift test를 관리합니다.

## 설치

```sh
go get github.com/awuzag/tossinvest-go
go install github.com/awuzag/tossinvest-go/cmd/tossinvest@latest
```

## SDK

```go
client, err := tossinvest.New(
	tossinvest.WithClientID(os.Getenv("TOSSINVEST_CLIENT_ID")),
	tossinvest.WithClientSecret(os.Getenv("TOSSINVEST_CLIENT_SECRET")),
)
if err != nil {
	log.Fatal(err)
}

prices, err := client.Prices(context.Background(), []string{"005930", "AAPL"})
```

가격, 금액, 수량은 decimal string으로 유지합니다. 반올림 정책을 호출자가 직접 책임지는 경우가 아니라면 `float`로 자동 변환하지 마세요.

## CLI

```sh
tossinvest --env-file .env.toss token
tossinvest --env-file .env.toss --json prices --symbols 005930,AAPL
tossinvest enable account
tossinvest --env-file .env.toss --json accounts
```

인증 정보는 명령줄 옵션, 환경 변수, env 파일에서 읽습니다. CLI는 secret 값을 출력하지 않습니다.
계좌/주문 기능 활성화 상태는 CLI 설정 파일에 저장합니다. 기본 위치는 OS 사용자 설정 디렉터리의 `tossinvest/config.json`이며, `--config` 또는 `TOSSINVEST_CONFIG`로 경로를 바꿀 수 있습니다.

지원하는 환경 변수:

| 변수 | 용도 |
| --- | --- |
| `TOSSINVEST_CLIENT_ID` 또는 `TOSS_CLIENT_ID` | OAuth client ID |
| `TOSSINVEST_CLIENT_SECRET` 또는 `TOSS_CLIENT_SECRET` | OAuth client secret |
| `TOSSINVEST_ACCESS_TOKEN` | 이미 발급받은 access token 재사용 |
| `TOSSINVEST_ACCOUNT_SEQ` | 기본 `X-Tossinvest-Account` 값 |

현재 awuzag 로컬 credential 파일 형식에 맞춰 repo-local env 파일에서는 `API_KEY`, `SCRET_KEY`도 사용할 수 있습니다.

## 지원 API

SDK는 OpenAPI 1.0.3의 전체 operation을 노출합니다.

- Auth: token 발급
- Market Data: 호가, 현재가, 체결, 상/하한가, 캔들
- Stock Info: 종목 기본 정보, 매수 유의사항
- Market Info: 환율, 국내/해외 장 운영 정보
- Account/Asset: 계좌 목록, 보유 주식
- Order History: 주문 목록, 주문 상세
- Order: 주문 생성, 정정, 취소
- Order Info: 매수 가능 금액, 판매 가능 수량, 수수료

주문 생성, 정정, 취소는 실제 거래 API입니다. SDK에는 구현되어 있지만, live e2e 테스트와 CLI 실행은 명시적인 opt-in이 있어야만 동작합니다.

## 기본 비활성화 정책

시장 데이터 조회는 기본으로 사용할 수 있지만, 계좌와 주문 관련 기능은 기본적으로 비활성화되어 있습니다.

- 계좌 관련 SDK 메서드: `WithAccountAPIsEnabled()`를 설정해야 합니다.
- 주문 조회/상세 SDK 메서드: `WithAccountAPIsEnabled()`와 `WithOrderAPIsEnabled()`를 함께 설정해야 합니다.
- 주문 생성/정정/취소 SDK 메서드: `WithAccountAPIsEnabled()`, `WithOrderAPIsEnabled()`, `WithLiveTradingEnabled()`를 모두 설정해야 합니다.
- CLI 계좌 명령: 먼저 `tossinvest enable account`를 실행해야 합니다.
- CLI 주문 조회/상세 명령: 먼저 `tossinvest enable account`와 `tossinvest enable orders`를 실행해야 합니다.
- CLI 주문 생성/정정/취소 명령: 추가로 `tossinvest enable live-trading confirm-live-orders`를 실행해야 합니다. 이 live-trading 활성화는 15분 뒤 만료됩니다.

현재 활성화 상태는 다음 명령으로 확인할 수 있습니다.

```sh
tossinvest features
```

## Contract 기준

`contracts/tossinvest/openapi.json`을 핵심 contract로 사용합니다. 검토된 OpenAPI 문서에서 provider-native request/response type, operation catalog metadata, raw API 호출 코드를 생성하고, public package와 CLI는 안정적인 중간 레이어로 유지합니다. 자세한 방향은 `docs/architecture/contract-codegen.md`를 참고하세요.

## 개발

```sh
go test ./...
go test ./... -cover
go test -tags=e2e ./...
```

기본 테스트는 fake HTTP server를 사용하며 Toss Invest live API를 호출하지 않습니다. Live e2e 테스트는 `e2e` build tag와 `TOSSINVEST_E2E=1`로 별도 활성화해야 합니다.
