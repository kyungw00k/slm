# SLM CLI 개편 계획서

## 개요
Switch Library Manager (SLM)를 현재의 GUI/Console 하이브리드 구조에서 순수 CLI 기반으로 개편하여 더 유연하고 확장 가능한 도구로 만드는 프로젝트입니다.

## 진행 상황 (2025-10-15)

### ✅ 완료된 주요 기능

1. **안정성 개선**
   - "close of closed channel" 패닉 해결
   - 원격 마운트 환경에서 안정적 동작 확보

2. **성능 최적화**
   - Worker Pool 기반 병렬 파일 처리 (2-5배 속도 향상)
   - 환경 감지 기반 적응형 워커 조절
   - 로컬 vs 원격 마운트 자동 감지 및 최적화

3. **UX 개선**
   - Discovery 단계 실시간 피드백 ("Found: N files")
   - 최근 발견 파일 표시
   - 환경 정보 표시 (파일시스템 타입)
   - 처리 속도 및 ETA 표시 준비 완료

### 🔄 진행 중

- 워커 통계 실시간 계산 (처리 속도, ETA)

### 📋 다음 단계

1. 워커 상태 표시 완성
2. 다중 폴더 병렬 스캔
3. 테스트 작성
4. 문서화

---

## 1. 핵심 변경사항

### 1.1 GUI 제거
- **현재**: Astilectron 기반 GUI + Console 모드
- **변경후**: 순수 CLI (`slm` 명령어)
- **이유**:
  - 의존성 대폭 감소 (Electron 런타임 제거)
  - 바이너리 크기 및 메모리 사용량 감소
  - 서버 환경 친화적
  - 유지보수성 향상

### 1.2 설정 디렉토리 표준화
- **현재**: 실행 파일과 같은 디렉토리에 설정/캐시
- **변경후**: `$HOME/.config/slm/` 또는 `$HOME/.slm/`

```
$HOME/.config/slm/
├── config.json              # 메인 설정 (기존 settings.json 호환)
├── cache/
│   ├── versions.json        # 버전 캐시
│   ├── titledb/
│   │   ├── KR.ko.json      # 언어별 타이틀 DB
│   │   ├── US.en.json
│   │   └── JP.ja.json
│   └── db/
│       ├── default.db       # 기본 로컬 DB (기존 구조 유지)
│       └── scan_{hash}.db   # 스캔 경로별 DB
├── logs/
│   └── slm.log             # 로그 파일
└── keys/
    └── prod.keys           # 암호화 키
```

### 1.3 DB 구조 유지
- 기존 BoltDB 구조 그대로 유지
- `LocalSwitchFilesDB`, `SwitchGameFiles` 등 모든 데이터 구조 호환
- 캐싱 메커니즘 유지

## 2. 새로운 CLI 구조

### 2.1 기본 명령어 체계
```bash
slm <command> [flags]
slm <command> <subcommand> [flags]
```

### 2.2 주요 명령어들

#### scan - 라이브러리 스캔 및 정리 (모든 핵심 기능 통합)
```bash
# 기본 스캔
slm scan -f /games                         # 스캔만 (check-all 기본값)
slm scan -F "/games1,/games2"              # 여러 폴더 (대문자 F)
slm scan -f /games --format json          # JSON 출력
slm scan -f /games --locale KR.ko         # 언어 지정
slm scan -f /games --no-recursive         # 비재귀 스캔

# 체크 옵션들
slm scan -f /games --check-updates        # 업데이트 누락만 체크
slm scan -f /games --check-dlc            # DLC 누락만 체크
slm scan -f /games --check-all            # 모든 누락 체크 (기본값)
slm scan -f /games --no-check             # 누락 체크 안함

# 정리 옵션들
slm scan -f /games --rename               # 스캔 후 파일명 변경
slm scan -f /games --create-folders       # 스캔 후 폴더별 정리
slm scan -f /games --delete-old-updates   # 스캔 후 구 업데이트 삭제
slm scan -f /games --rename --create-folders  # 조합 가능

# 미리보기
slm scan -f /games --rename --dry-run     # 변경사항 미리보기
```

#### config - 설정 관리 (단순화)
```bash
slm config                                # 현재 설정 보기
slm config <key>                          # 특정 값 조회
slm config <key> <value>                  # 값 설정
slm config --path                         # 설정 파일 경로
slm config --migrate                      # 기존 settings.json 변환
```

#### cache - 캐시 관리 (단순화)
```bash
slm cache                                 # 캐시 상태 보기
slm cache --clean                         # 캐시 삭제
slm cache --update                        # 강제 업데이트
slm cache --path                          # 캐시 디렉토리 경로
```

### 2.3 공통 플래그들
```bash
# 필수 플래그들
-f, --folder <path>         # 스캔할 폴더 (모든 주요 명령어)
-F, --folders <path1,path2> # 여러 폴더 (콤마 구분)
-h, --help                  # 도움말 (모든 명령어)
-v, --verbose               # 상세 출력
-q, --quiet                 # 최소 출력

# 스캔 관련
-r, --recursive             # 재귀 스캔 (기본값)
--no-recursive              # 재귀 스캔 비활성화
--format <table|json|csv>   # 출력 형식
--locale <KR.ko|US.en|JP.ja> # 언어 설정

# 정리 관련
--dry-run                   # 변경사항 미리보기
--rename                    # 파일명 변경
--create-folders            # 게임별 폴더 생성
--delete-old-updates        # 구 업데이트 파일 삭제
--template <name|path>      # 사용자 정의 템플릿

# 체크 관련
--updates                   # 업데이트 체크
--dlc                       # DLC 체크
--all                       # 모든 체크 (기본값)
--ignore-dlc <id1,id2>      # 특정 DLC 무시

# 전역 플래그들
--config-dir <path>         # 설정 디렉토리 지정
--no-cache                  # 캐시 사용 안함
--output-mode <auto|simple|rich|json|csv>  # 출력 모드
```

## 3. 진행률 표시 개선 ✅ 완료

### 3.1 구현된 개선사항

**Discovery 단계 피드백**:
```bash
🔄 [1/6] Scanning files...  ⠋  Found: 1,234 files
      Recent:
      ├─ Super Mario Odyssey.nsp
      ├─ Pokemon Sword.nsp
      └─ Zelda BOTW.nsp
```

**Processing 단계 피드백**:
```bash
🔄 [2/6] Processing files... [▓▓▓▓▓▓▓▓░░] 342/1,234 (28%)
      Environment: Local (apfs)
      Workers: 10 total | Rate: 23.5 files/sec | ETA: 2m 15s
```

### 3.2 향후 개선 가능 사항
- 개별 워커 상태 표시 (현재 처리 중인 파일)
- 워커별 진행률 표시
- 더 상세한 처리 단계 표시 (decrypting, parsing, etc.)

## 4. 설정 우선순위
1. CLI 플래그 (최우선)
2. 환경변수 (`SLM_CONFIG_DIR`, `SLM_CACHE_DIR` 등)
3. `$HOME/.config/slm/config.json`
4. `$HOME/.slm/config.json`
5. 기본값

## 5. 기존 호환성 유지

### 5.1 데이터 구조
- 모든 기존 Go 구조체 유지
- BoltDB 스키마 변경 없음
- 기존 캐시 및 데이터베이스 파일 재사용 가능

### 5.2 설정 마이그레이션
- 기존 `settings.json` → 새 `config.json` 자동 변환
- 기존 키 파일 경로 지원 (`${HOME}/.switch/prod.keys`)

### 5.3 기능 동등성
- 현재 console.go의 모든 기능 지원
- GUI에서만 가능했던 설정 편집도 CLI로 제공

## 6. 구현 단계별 계획

### Phase 1: 기본 CLI 구조
1. 새 main.go 작성 (cobra CLI 프레임워크 사용)
2. 기본 명령어 구조 구현
3. 설정 디렉토리 및 마이그레이션 로직

### Phase 2: 핵심 기능 이식
1. scan 명령어 (기존 console.go 로직 활용)
2. organize 명령어
3. check 명령어

### Phase 3: 고급 기능
1. 개선된 진행률 시스템 (Bubble Tea)
2. 대화형 설정 편집
3. 사용자 정의 템플릿 지원

### Phase 4: 최적화 및 문서화
1. 성능 최적화
2. 에러 처리 개선
3. 사용자 문서 작성

## 7. 기술적 고려사항

### 7.1 사용할 라이브러리
- **CLI**: github.com/spf13/cobra
- **TUI**: github.com/charmbracelet/bubbletea + github.com/charmbracelet/bubbles
  - 진행률: bubbles/progress
  - 스피너: bubbles/spinner
  - 리스트: bubbles/list
  - 입력: bubbles/textinput
  - 스타일링: github.com/charmbracelet/lipgloss
- **설정**: 기존 JSON 유지 (golang encoding/json)
- **로깅**: 기존 zap 유지

### 7.2 빌드 및 배포
- 단일 바이너리 배포
- 크로스 플랫폼 지원 (Windows, macOS, Linux)
- GitHub Actions를 통한 자동 빌드

### 7.3 테스트 전략
- 기존 기능과의 호환성 테스트
- 다양한 게임 라이브러리 구조 테스트
- 성능 회귀 테스트

## 8. 예상 효과

### 8.1 사용자 경험 개선
- 더 직관적이고 유연한 CLI 인터페이스
- 명확한 진행률 및 피드백
- 스크립팅 및 자동화 지원

### 8.2 개발 및 유지보수성
- 단순화된 아키텍처
- 테스트 용이성 증가
- 의존성 감소로 인한 안정성 향상

### 8.3 성능 개선
- 메모리 사용량 대폭 감소
- 바이너리 크기 감소
- 서버 환경에서의 효율성 증가