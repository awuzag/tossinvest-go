# Toss Invest Contract

`openapi.json`은 이 저장소가 사용하는 검토 완료 contract snapshot입니다.

- upstream: `https://openapi.tossinvest.com/openapi-docs/latest/openapi.json`
- package: `github.com/awuzag/tossinvest-go`
- status: checked-in contract, 자동 갱신하지 않음

이 파일은 일반 문서가 아니라 code generation 기준입니다. Generated request/response type, operation catalog metadata, raw API 호출, CLI metadata는 이 contract에서 파생하고, public `tossinvest` package는 안정적인 중간 레이어로 유지합니다.

갱신 정책:

1. upstream OpenAPI 문서를 임시 파일로 내려받습니다.
2. operation, schema, auth, account header, order risk 변경을 검토합니다.
3. 검토가 끝난 뒤에만 `openapi.json`을 교체합니다.
4. 코드를 다시 생성하고 contract drift test를 실행합니다.
