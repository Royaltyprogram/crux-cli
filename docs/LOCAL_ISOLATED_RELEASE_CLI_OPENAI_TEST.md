# Local Isolated Release CLI + OpenAI Test

이 문서는 아래를 한 번에 검증하기 위한 최소 명령 세트다.

- 로컬에 이미 설치된 배포 CLI와 섞이지 않는 별도 테스트 CLI 설치
- 로컬 서버가 실제 OpenAI 호출로 리포트를 생성하는지 확인
- 테스트 중 서버 로그에 File ID 관련 오류가 없는지 확인

전제:

- 현재 디렉터리는 repo root
- Python 명령은 `source myenv/bin/activate` 후 실행
- 필요한 시크릿 값은 모두 "파일"로 이미 존재함
- 기존 `~/.autoskills`, `~/.local/bin/autoskills` 는 건드리지 않음

## 1. 공통 테스트 경로 준비

```bash
export TEST_ROOT="$PWD/.autoskills-localtest"
export TEST_SECRET_DIR="$TEST_ROOT/secrets"
export TEST_RUNTIME_DIR="$TEST_ROOT/runtime"
export TEST_INSTALL_ROOT="$TEST_ROOT/install"
export TEST_BIN_DIR="$TEST_ROOT/bin"
export TEST_CLI_HOME="$TEST_ROOT/cli-home"
export TEST_SESSION_DIR="$TEST_ROOT/session-fixtures"
export TEST_COOKIE_JAR="$TEST_ROOT/google.cookies.txt"
export TEST_SERVER_LOG="$TEST_ROOT/server.log"
export TEST_DB_DSN="$TEST_RUNTIME_DIR/autoskills-local.db?_fk=1"
export TEST_BASE_URL="http://127.0.0.1:8082"
export TEST_RELEASE_VERSION="${TEST_RELEASE_VERSION:-0.1.1-beta}"

rm -rf "$TEST_ROOT"
mkdir -p \
  "$TEST_SECRET_DIR" \
  "$TEST_RUNTIME_DIR" \
  "$TEST_INSTALL_ROOT" \
  "$TEST_BIN_DIR" \
  "$TEST_CLI_HOME" \
  "$TEST_SESSION_DIR"
```

## 2. 테스트용 시크릿 생성

기존 파일 기반 시크릿을 테스트 전용 디렉터리로 복사해 사용한다. 아래 경로는 필요하면 실제 보관 위치로 바꿔서 쓴다.

```bash
source myenv/bin/activate

export SRC_JWT_SECRET_FILE="${SRC_JWT_SECRET_FILE:-secrets/autoskills-jwt-secret}"
export SRC_OPENAI_API_KEY_FILE="${SRC_OPENAI_API_KEY_FILE:-secrets/autoskills-openai-api-key}"
export SRC_BOOTSTRAP_USERS_FILE="${SRC_BOOTSTRAP_USERS_FILE:-secrets/autoskills-beta-users.json}"

test -f "$SRC_JWT_SECRET_FILE"
test -f "$SRC_OPENAI_API_KEY_FILE"
test -f "$SRC_BOOTSTRAP_USERS_FILE"

cp "$SRC_JWT_SECRET_FILE" "$TEST_SECRET_DIR/jwt-secret"
cp "$SRC_OPENAI_API_KEY_FILE" "$TEST_SECRET_DIR/openai-api-key"
cp "$SRC_BOOTSTRAP_USERS_FILE" "$TEST_SECRET_DIR/bootstrap-users.json"
```

## 3. 터미널 1: 로컬 서버 시작

이 터미널은 켜둔다.

```bash
source myenv/bin/activate

APP_MODE=local \
JWT_SECRET_FILE="$TEST_SECRET_DIR/jwt-secret" \
AUTH_BOOTSTRAP_USERS_FILE="$TEST_SECRET_DIR/bootstrap-users.json" \
OPENAI_API_KEY_FILE="$TEST_SECRET_DIR/openai-api-key" \
OPENAI_RESPONSES_MODEL=gpt-5.4 \
DB_DSN="$TEST_DB_DSN" \
DB_DIALECT=sqlite3 \
HTTP_LOG_TO_STDOUT=true \
GOOGLE_STUB_EMAIL=beta1@example.com \
GOOGLE_STUB_NAME="Beta Operator" \
./scripts/run_local_google_stub.sh 2>&1 | tee "$TEST_SERVER_LOG"
```

## 4. 터미널 2: 서버 준비 대기 + 로그인 쿠키 + CLI 토큰 발급

```bash
source myenv/bin/activate

for _ in $(seq 1 30); do
  if curl -fsS "$TEST_BASE_URL/healthz" >/dev/null && curl -fsS "$TEST_BASE_URL/readyz" >/dev/null; then
    break
  fi
  sleep 1
done

curl -fsS -L \
  -c "$TEST_COOKIE_JAR" \
  -b "$TEST_COOKIE_JAR" \
  "$TEST_BASE_URL/api/v1/auth/google/start" \
  >/dev/null

export TEST_CLI_TOKEN="$(
  curl -fsS \
    -c "$TEST_COOKIE_JAR" \
    -b "$TEST_COOKIE_JAR" \
    -H 'Content-Type: application/json' \
    -d '{"label":"isolated-local-openai-test"}' \
    "$TEST_BASE_URL/api/v1/auth/cli-tokens" \
  | python -c 'import json,sys; payload=json.load(sys.stdin); print((payload.get("data") or {}).get("token","").strip())'
)"

test -n "$TEST_CLI_TOKEN"
printf '%s\n' "$TEST_CLI_TOKEN"
```

## 5. 터미널 2: 격리된 배포 CLI 설치

이 단계는 기존 `~/.local/bin/autoskills` 를 덮어쓰지 않는다.

```bash
AUTOSKILLS_VERSION="$TEST_RELEASE_VERSION" \
AUTOSKILLS_INSTALL_ROOT="$TEST_INSTALL_ROOT" \
AUTOSKILLS_BIN_DIR="$TEST_BIN_DIR" \
AUTOSKILLS_AUTO_PATH=never \
./scripts/install.sh

export TEST_CLI="$TEST_BIN_DIR/autoskills"

AUTOSKILLS_HOME="$TEST_CLI_HOME" "$TEST_CLI" version
```

## 6. 터미널 2: 격리된 CLI로 로그인/워크스페이스 연결

```bash
AUTOSKILLS_HOME="$TEST_CLI_HOME" "$TEST_CLI" login \
  --server "$TEST_BASE_URL" \
  --token "$TEST_CLI_TOKEN" \
  --device "isolated-local-openai-test" \
  --hostname "isolated-local-openai-test.local" \
  --platform "manual/local-test"

AUTOSKILLS_HOME="$TEST_CLI_HOME" "$TEST_CLI" connect \
  --repo-path "$PWD"

AUTOSKILLS_HOME="$TEST_CLI_HOME" "$TEST_CLI" snapshot \
  --file examples/config-snapshot.json
```

## 7. 터미널 2: 리포트 생성용 세션 10개 준비

서버는 첫 리포트 발행 전에 최소 10개 세션을 본다. 아래는 예제 세션을 10개로 복제하면서 `session_id` 와 `timestamp` 를 다르게 만든다.

```bash
source myenv/bin/activate

python - <<'PY'
import json
import pathlib
from datetime import datetime, timedelta, timezone

root = pathlib.Path(".autoskills-localtest/session-fixtures")
template = json.loads(pathlib.Path("examples/session-summary.json").read_text())
base = datetime(2026, 3, 10, 8, 0, 0, tzinfo=timezone.utc)

for idx in range(10):
    payload = dict(template)
    payload["session_id"] = f"isolated-session-{idx+1:02d}"
    payload["timestamp"] = (base + timedelta(minutes=idx)).isoformat().replace("+00:00", "Z")
    payload["raw_queries"] = [
        f"[{idx+1:02d}] Find the analytics route that is failing and explain the current control flow.",
        f"[{idx+1:02d}] Check whether the health controller and analytics controller share the same response contract.",
        f"[{idx+1:02d}] Draft the smallest patch that fixes the regression and list the exact tests to run.",
    ]
    (root / f"session-{idx+1:02d}.json").write_text(json.dumps(payload, indent=2) + "\n")
PY
```

## 8. 터미널 2: 세션 업로드 + 리포트 생성 확인

```bash
for file in "$TEST_SESSION_DIR"/session-*.json; do
  AUTOSKILLS_HOME="$TEST_CLI_HOME" "$TEST_CLI" session --file "$file"
done

for _ in $(seq 1 30); do
  AUTOSKILLS_HOME="$TEST_CLI_HOME" "$TEST_CLI" reports > "$TEST_ROOT/reports.json"
  if python - <<'PY'
import json
import pathlib
payload = json.loads(pathlib.Path(".autoskills-localtest/reports.json").read_text())
data = payload.get("data") if isinstance(payload, dict) and "code" in payload else payload
items = (data or {}).get("items") or []
raise SystemExit(0 if items else 1)
PY
  then
    break
  fi
  sleep 2
done

cat "$TEST_ROOT/reports.json"
AUTOSKILLS_HOME="$TEST_CLI_HOME" "$TEST_CLI" status | tee "$TEST_ROOT/status.json"
```

## 9. 터미널 2: OpenAI 응답 모드 확인

첫 리포트 evidence 안에 `generation_mode=openai_responses_api` 가 있어야 한다.

```bash
source myenv/bin/activate

python - <<'PY'
import json
import pathlib

payload = json.loads(pathlib.Path(".autoskills-localtest/reports.json").read_text())
data = payload.get("data") if isinstance(payload, dict) and "code" in payload else payload
items = (data or {}).get("items") or []
if not items:
    raise SystemExit("reports missing items")
evidence = items[0].get("evidence") or []
needle = "generation_mode=openai_responses_api"
if needle not in evidence:
    raise SystemExit(f"missing {needle}: {evidence}")
print("OpenAI report generation verified")
PY
```

## 10. 터미널 2: File ID 관련 오류가 없는지 로그 확인

현재 리포트 생성 경로는 Responses API에 문자열 prompt를 직접 보내므로, 아래 grep은 File ID 관련 회귀가 없는지 확인하는 런타임 가드다.

```bash
if rg -n -i 'file[^a-z0-9]*id|invalid file|no such file|file not found' "$TEST_SERVER_LOG"; then
  echo "unexpected File ID related log found"
  exit 1
else
  echo "no File ID issue found in server log"
fi
```

## 11. 정리

```bash
rm -rf "$TEST_ROOT"
```
