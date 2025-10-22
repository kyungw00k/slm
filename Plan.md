# SLM CLI 개편 계획서

## 개요
Switch Library Manager (SLM)를 현재의 GUI/Console 하이브리드 구조에서 순수 CLI 기반으로 개편하여 더 유연하고 확장 가능한 도구로 만드는 프로젝트입니다.

## 진행 상황 (2025-10-22)

### ✅ 완료된 주요 기능

1. **안정성 개선**
   - "close of closed channel" 패닉 해결
   - 원격 마운트 환경에서 안정적 동작 확보

2. **성능 최적화 (85.6% 성능 향상)**
   - Worker Pool 기반 병렬 파일 처리 (2-5배 속도 향상)
   - 환경 감지 기반 적응형 워커 조절
   - 로컬 vs 원격 마운트 자동 감지 및 최적화
   - Atomic CAS 패턴으로 Progress 동시성 문제 해결

3. **UX 개선**
   - Discovery 단계 실시간 피드백 ("Found: N files")
   - 최근 발견 파일 표시
   - 환경 정보 표시 (파일시스템 타입)
   - stderr 출력 + Sync()로 progress buffering 문제 해결

4. **scan/list 명령어 분리** ✅
   - scan: DB 업데이트 및 요약 정보 (--show-table 옵션)
   - list: DB 조회, 필터링, 정렬, 페이징 기능

5. **Multi-language TitleDB 지원** ✅
   - 우선순위: KR.ko > JP.ja > US.en
   - 한글 제목 우선 표시
   - Filename parsing fallback

6. **TitleDB 매칭 수정** ✅ **오늘 수정**
   - JSON key(14자리 decimal)가 아닌 id 필드(16자리 hex) 사용
   - 한글/일본어 제목 정상 표시
   - 빈 id 필드 처리 (5,138 / 31,437 엔트리)

### 📋 다음 단계

1. 전체 스캔 테스트 및 성능 검증
2. 테스트 작성 확대
3. 문서화 업데이트 (README.md)

### 문제점
현재 `scan` 명령어가 스캔과 결과 출력을 동시에 수행하고 있어 다음과 같은 문제가 있습니다:

1. **출력 가독성 문제**: 3000개 이상의 타이틀이 있는 경우 테이블이 너무 길어서 확인이 어려움
2. **기능 혼재**: 스캔(DB 업데이트)과 조회(결과 확인)가 하나의 명령어에서 처리됨
3. **재조회 불가**: 스캔 없이 기존 DB 내용만 조회하고 싶을 때 불편

### 해결 방안: scan과 list 명령어 분리

#### 1. `scan` 명령어 - 스캔 및 DB 업데이트
**역할**: 게임 파일을 스캔하여 DB를 업데이트하는 것에 집중

**출력 개선**:
```
$ slm-new scan -f /Volumes/roms/SWITCH

Switch Library Manager

[1/4] Download DBs
[1/4] Download DBs (completed)
[2/4] Scan files
[2/4] Scan files 1234 files (completed)
[3/4] Build library
[3/4] Build library (completed)
[4/4] Check missing
[4/4] Check missing (completed)

Summary:
  Total games: 3,245
  - Base games: 2,890
  - Updates: 1,234
  - DLC: 567

  Missing content:
  - Updates: 234 games
  - DLC: 456 games

Scan completed in 2m 34s
Database updated: ~/.config/slm/cache/db/scan_XXXXXXXXX.db
```

**주요 변경사항**:
- 기본적으로 테이블 출력 제거
- 요약 정보만 표시 (총 게임 수, 업데이트/DLC 현황, 누락된 콘텐츠)
- `--show-table` 플래그로 테이블 출력 옵션 제공 (짧은 리스트용)

#### 2. `list` 명령어 - DB 조회 및 표시 (새로운 명령어)
**역할**: 기존 DB를 조회하여 다양한 형태로 결과 표시

**기본 사용법**:
```bash
# 전체 목록 조회 (페이징)
$ slm-new list

# 특정 조건 필터링
$ slm-new list --missing-updates    # 업데이트가 없는 게임만
$ slm-new list --missing-dlc         # DLC가 없는 게임만
$ slm-new list --title "zelda"       # 제목 검색
$ slm-new list --limit 20            # 상위 20개만

# 다양한 출력 형식
$ slm-new list --format table        # 테이블 형식 (기본)
$ slm-new list --format json         # JSON 형식
$ slm-new list --format csv          # CSV 형식

# 정렬
$ slm-new list --sort title          # 제목순
$ slm-new list --sort title-id       # TitleID순
$ slm-new list --sort missing        # 누락된 콘텐츠 많은 순

# 페이징
$ slm-new list --page 2 --per-page 50
```

**구현 계획**:

1. **새로운 파일 생성**: `cmd/list.go`
   ```go
   package cmd

   var listCmd = &cobra.Command{
       Use:   "list",
       Short: "List games from the database",
       Long:  "Query and display games from the local database with various filters and formats",
       Run:   runList,
   }

   func init() {
       rootCmd.AddCommand(listCmd)

       // Filters
       listCmd.Flags().Bool("missing-updates", false, "Show only games missing updates")
       listCmd.Flags().Bool("missing-dlc", false, "Show only games missing DLC")
       listCmd.Flags().StringP("title", "t", "", "Filter by title (case-insensitive substring match)")
       listCmd.Flags().StringP("title-id", "i", "", "Filter by title ID")
       listCmd.Flags().IntP("limit", "l", 0, "Limit number of results (0 = no limit)")

       // Sorting
       listCmd.Flags().StringP("sort", "s", "title", "Sort by: title|title-id|missing")

       // Pagination
       listCmd.Flags().Int("page", 1, "Page number (starts from 1)")
       listCmd.Flags().Int("per-page", 50, "Results per page")

       // Output format
       listCmd.Flags().StringP("format", "f", "table", "Output format: table|json|csv")
   }
   ```

2. **DB 조회 함수 추가**: `db/localSwitchFilesDB.go`
   ```go
   type ListOptions struct {
       MissingUpdates bool
       MissingDLC     bool
       TitleFilter    string
       TitleIDFilter  string
       Limit          int
       SortBy         string
       Page           int
       PerPage        int
   }

   func (ldb *LocalSwitchDBManager) ListGames(opts ListOptions) ([]*SwitchGameFiles, error) {
       // DB에서 게임 목록 조회
       // 필터 적용
       // 정렬
       // 페이징
       return games, nil
   }
   ```

3. **`scan` 명령어 수정**: `cmd/scan.go`
   ```go
   // --show-table 플래그 추가
   scanCmd.Flags().Bool("show-table", false, "Show full table after scan (not recommended for large libraries)")

   // 기본 동작: 요약만 표시
   // --show-table 사용 시에만 테이블 출력
   ```

#### 3. 추가 편의 기능

**stats 명령어** (옵션):
```bash
$ slm-new stats

Switch Library Statistics
========================
Total Games: 3,245
  - Base games: 2,890
  - Updates: 1,234
  - DLC: 567

Missing Content:
  - Games missing updates: 234 (8.1%)
  - Games missing DLC: 456 (15.8%)

Top 5 Games with Most DLC:
  1. Pokemon Sword/Shield - 24 DLC
  2. Fire Emblem Three Houses - 18 DLC
  3. Animal Crossing - 15 DLC
  ...

Storage:
  Total size: 1.2 TB
  Average game size: 4.3 GB
```

### 작업 순서

1. ✅ 진행 상황 표시 문제 해결 (go-expert에게 위임)
   - ✅ atomic CAS 패턴으로 중복 업데이트 방지
   - ✅ stderr 출력 및 명시적 버퍼 flush
   - ✅ 테스트 코드 작성 및 검증

2. ✅ `list` 명령어 구현
   - ✅ `cmd/list.go` 생성
   - ✅ DB 조회 함수 구현 (ListGames)
   - ✅ 필터링 로직 구현 (title, title-id, missing-updates, missing-dlc)
   - ✅ 페이징 구현 (page, per-page)
   - ✅ 정렬 지원 (title, title-id, missing)
   - ✅ 자동 DB 탐지 (가장 최근 DB)
   - ✅ --scan-path 옵션 추가

3. ✅ `scan` 명령어 수정
   - ✅ 기본 출력을 요약으로 변경
   - ✅ `--show-table` 플래그 추가
   - ✅ 요약 정보 출력 (게임 수, 업데이트/DLC, 누락 콘텐츠)

4. ✅ 테스트 및 문서 업데이트
   - ✅ 각 명령어 테스트 (빈 DB, 필터, 페이징, JSON/CSV)
   - ✅ CLAUDE.md 업데이트
   - [ ] README 업데이트 (나중에)

### 완료 (2025-10-22)

모든 작업이 완료되었습니다!

**주요 개선사항**:
- 진행 상황 표시: atomic CAS 패턴으로 들쭉날쭉한 숫자 문제 해결
- scan/list 분리: 대용량 라이브러리에서 깔끔한 UX 제공
- 다양한 필터/정렬 옵션으로 유연한 조회 가능
- JSON/CSV 출력으로 스크립팅 지원

**커밋**:
- `40190b2` - perf: fix duplicate progress updates using atomic CAS pattern
- `a3a9a86` - feat: separate scan and list commands for better UX

---

## 이전 작업: 파일 스캔 진행 상황 개선 (2025-10-21)

### 문제점
네트워크 파일 시스템에서 파일 탐색 시 진행 상황이 불명확:
- Stage 2 (Scan files)에서 10초 이상 소요되는데 진행률이 보이지 않음
- `filepath.Walk()`가 순차적으로 동작하며 current/total이 -1로 고정
- 사용자가 프로그램이 멈춘 것인지 작동 중인지 알 수 없음

### 해결 방안: 발견된 파일 수 실시간 표시

**구현 계획:**

1. **`scanFolder` 함수 개선** (db/localSwitchFilesDB.go:160-191)
   ```go
   // 발견된 파일 수를 실시간으로 업데이트
   filesFound := 0
   filepath.Walk(folder, func(path string, info os.FileInfo, err error) error {
       if shouldIncludeFile(info) {
           filesFound++
           *files = append(*files, ExtendedFileInfo{...})

           // 10개마다 진행 상황 업데이트
           if progress != nil && (filesFound % 10 == 0) {
               progress.UpdateProgress(1, filesFound, 0,
                   fmt.Sprintf("Found %d files", filesFound),
                   "Current: " + truncateFilename(info.Name(), 40))
           }
       }
   })
   ```

2. **폴더별 진행 상황 표시** (db/localSwitchFilesDB.go:136-144)
   ```go
   // 여러 폴더 스캔 시 폴더별 진행도 표시
   for i, folder := range folders {
       updater.UpdateProgress(1, i, len(folders),
           fmt.Sprintf("Scanning folder %d/%d", i+1, len(folders)),
           filepath.Base(folder))

       scanFolder(folder, recursive, &files, progress)
   }
   ```

3. **환경 정보 표시** (pkg/scanner/scanner.go:316-387)
   ```go
   // 사용자에게 환경 및 워커 정보 제공
   envInfo := fmt.Sprintf("Environment: %s | Workers: %d", envStr, numWorkers)
   updater.UpdateProgress(1, 0, 0, "Starting file scan...", envInfo)
   ```

### 예상 결과

**Before:**
```
Stage 2: Scan files - Initializing file scanner
```
(아무 변화 없이 10초 경과...)

**After:**
```
Stage 2: Scan files
Environment: NFS (nfs4) | Workers: 24
[▓▓▓▓▓▓░░░░░░░░] Scanning folder 1/3: Games
Found 1,247 files | Current: Super Mario Odyssey.nsp

[████████████░░] Scanning folder 2/3: Updates
Found 2,891 files | Current: Zelda BOTW v131072.nsp

[██████████████] Scan complete: 4,523 files found
```

### 성공 기준
- [✅] 파일 발견 개수 실시간 표시 (10개마다 업데이트)
- [✅] 현재 스캔 중인 파일명 표시 (40자로 truncate)
- [✅] 여러 폴더 스캔 시 폴더별 진행률
- [✅] 네트워크 환경 정보 표시 (환경 타입 + 워커 수)

### 구현 완료 (2025-10-21)

**진행 상황 표시 개선:**
1. `db/localSwitchFilesDB.go`
   - `scanFolder()`: 파일 발견 수 실시간 카운팅 및 진행 상황 업데이트
   - `CreateLocalSwitchFilesDB()`: 폴더별 진행 상황 표시 개선
2. `pkg/scanner/scanner.go`
   - `scanFiles()`: 환경 정보 및 워커 수 표시 추가

**Phase 1 성능 최적화 (2025-10-22):**
1. `pkg/scanner/output.go`
   - 한국어 우선 설정을 Config에서 읽어오기 (`s.settings.LocalePriority`)
2. `db/localSwitchFilesDB.go`
   - 조기 필터링: 확장자 체크를 스캔 중 즉시 수행
   - godirwalk 사용: 64KB 버퍼로 네트워크 I/O 최적화
3. `go.mod`
   - `github.com/karrick/godirwalk v1.17.0` 추가

**성능 개선 결과:**
- **시간**: 2-3분 → **26.7초** (약 **85% 단축!** ⚡)
- **메모리**: 조기 필터링으로 불필요한 파일 제외 (1,491개 전체 파일 → 607개 게임 파일)
- **한국어 지원**: Config의 locale_priority 반영 완료
- **다국어 타이틀**: 한국어, 일본어, 영어 모두 정상 표시
  - 예: "한국 드론 플라잉 투어 제주도", "팩맨 월드 2 리팩", "ボイスラブオンエア", "NekoRamen"
- **진행 표시**: 실시간 파일 카운트 업데이트 (100/1491 → 1400/1491)

**Phase 2 성능 최적화 (2025-10-22):**
1. `db/localSwitchFilesDB.go`
   - 폴더별 병렬 스캔: 여러 폴더를 goroutine으로 동시 처리
   - 스트리밍 파이프라인: 채널 기반 파일 스트리밍으로 메모리 효율 개선
   - 적응형 워커 최적화: CPU 코어 수와 폴더 수에 따라 자동 조절

**Phase 2 성능 개선 결과:**
- **시간**: 105.9초 → **15.3초** (약 **85.6% 단축!** ⚡⚡)
- **전체 개선**: 기존 대비 **총 6.9배 성능 향상**
- **병렬 처리**: 효과적인 I/O 대기 시간 활용
- **대용량 테스트**: 네트워크 볼륨 재귀 스캔 성공 (3분 36초, 607 게임)

**에러 처리 및 코드 품질 개선 (2025-10-22):**
1. `db/localSwitchFilesDB.go`
   - ScanErrors 구조체: 스레드 안전한 에러 수집 시스템
   - 폴더 레벨 에러 추적: 폴더별 스캔 실패 원인 기록
   - 파일 레벨 에러 추적: 개별 파일 처리 에러와 단계 정보 저장
   - 적응형 버퍼 크기: 워커 수와 폴더 수에 따른 동적 계산
   - Summary() 메서드: 사용자 친화적 에러 요약 생성

2. `pkg/scanner/scanner.go`
   - 에러 요약 표시: 스캔 완료 후 에러 통계 및 상세 정보 출력
   - 미사용 파라미터 정리: underscore 컨벤션으로 코드 의도 명확화
   - 진단 경고 해결: 모든 linter 경고 수정 완료

**에러 처리 기능:**
- **스레드 안전**: Mutex를 사용한 동시성 보장
- **에러 분류**: 폴더 에러와 파일 에러 분리 추적
- **단계 정보**: 에러 발생 위치 (scan/process/metadata) 기록
- **사용자 피드백**: 스캔 후 에러 개수와 요약 자동 표시
- **적응형 버퍼**: `min(workers * folders, 1000)` 캡으로 메모리 효율 유지

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