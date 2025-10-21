# Switch Library Manager - 성능 최적화 방안

## 현재 성능 분석 (2025-10-21)

### 테스트 환경
- **볼륨**: `/Volumes/roms/SWITCH` (네트워크 마운트)
- **파일 수**: 1,491개 (비재귀 스캔)
- **스캔 시간**: 약 2-3분

### 성능 병목 분석

#### Stage 2: Scan files (파일 탐색)
```
[2/4] Scan files 100/1491 (7%)
...
[2/4] Scan files 1400/1491 (94%)
```

**현재 구조**:
- `filepath.Walk()` 사용: **단일 스레드 순차 탐색**
- 각 디렉토리 읽기마다 네트워크 RTT 발생
- 1,491개 파일 탐색에 약 30-60초 소요

**병목 원인**:
1. 순차적 디렉토리 순회
2. 네트워크 레이턴시 누적
3. I/O 대기 시간 동안 CPU 유휴

#### Stage 3: Build library (메타데이터 처리)
```
[3/4] Build library (completed)
```

**현재 구조**:
- Worker Pool 사용: **병렬 처리**
- 네트워크 환경: CPU 코어 × 2-3배 워커
- 1,491개 파일 처리에 약 1-2분 소요

**상태**: ✅ 이미 최적화됨

---

## 성능 개선 방안

### 🚀 방안 1: 파일 탐색 병렬화 (High Impact)

**현재 문제**:
```go
filepath.Walk(folder, walkFunc)  // 단일 스레드
```

**개선 방안**: 디렉토리 레벨별 병렬 탐색

```go
// 1단계: 최상위 서브디렉토리 리스트 수집
subdirs := listTopLevelDirs(folder)

// 2단계: 각 서브디렉토리를 병렬로 탐색
var wg sync.WaitGroup
filesChan := make(chan ExtendedFileInfo, 1000)

for _, subdir := range subdirs {
    wg.Add(1)
    go func(dir string) {
        defer wg.Done()
        filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
            // ... 파일 수집 ...
            filesChan <- ExtendedFileInfo{...}
        })
    }(subdir)
}

// 3단계: 결과 수집
go func() {
    wg.Wait()
    close(filesChan)
}()

for file := range filesChan {
    *files = append(*files, file)
}
```

**예상 효과**:
- 탐색 시간: **50-70% 감소** (30-60초 → 10-20초)
- 네트워크 I/O 병렬화로 RTT 중복 활용

**구현 난이도**: 중간

---

### 🔥 방안 2: 파일 시스템 읽기 버퍼 증가 (Low Impact, Easy)

**현재 문제**:
- Go의 기본 `filepath.Walk()`는 작은 버퍼 사용
- 네트워크 파일 시스템에서 비효율적

**개선 방안**: `readdir` 배치 크기 증가

```go
// 커스텀 Walk 함수 사용 (godirwalk 라이브러리)
import "github.com/karrick/godirwalk"

godirwalk.Walk(folder, &godirwalk.Options{
    Callback: func(osPathname string, de *godirwalk.Dirent) error {
        // ... 파일 처리 ...
    },
    ScratchBuffer: make([]byte, 64*1024), // 64KB 버퍼
    Unsorted: true, // 정렬 불필요
})
```

**예상 효과**:
- 네트워크 I/O 횟수: **30-50% 감소**
- 탐색 시간: **10-20% 감소**

**구현 난이도**: 쉬움

---

### ⚡ 방안 3: 캐시 개선 (Medium Impact)

**현재 문제**:
- BoltDB 캐시가 있지만 파일 변경 감지 없음
- 매번 전체 파일 탐색 수행

**개선 방안**: 증분 스캔 (Incremental Scan)

```go
// 1. 이전 스캔 결과 로드
previousScan := loadPreviousScan(scanPath)

// 2. 파일 시스템 변경 감지
changes := detectChanges(folder, previousScan)

// 3. 변경된 파일만 재스캔
for _, file := range changes.Added {
    scanFile(file)
}

for _, file := range changes.Modified {
    rescanFile(file)
}

for _, file := range changes.Deleted {
    removeFromDB(file)
}
```

**파일 변경 감지 방법**:
- 파일 mtime (수정 시간) 비교
- 파일 크기 비교
- 디렉토리 구조 해시

**예상 효과**:
- 재스캔 시: **90% 이상 시간 단축**
- 초기 스캔: 영향 없음

**구현 난이도**: 중간-높음

---

### 🎯 방안 4: 조기 필터링 (Low Impact, Easy)

**현재 문제**:
```go
// 모든 파일을 리스트에 추가한 후 필터링
*files = append(*files, ExtendedFileInfo{...})
```

**개선 방안**: 탐색 중 즉시 필터링

```go
// 확장자 체크를 Walk 내부에서 수행
fileName := strings.ToLower(info.Name())
if !strings.HasSuffix(fileName, ".nsp") &&
   !strings.HasSuffix(fileName, ".nsz") &&
   !strings.HasSuffix(fileName, ".xci") &&
   !strings.HasSuffix(fileName, ".xcz") {
    return nil  // 즉시 스킵
}

*files = append(*files, ExtendedFileInfo{...})
```

**예상 효과**:
- 메모리 사용량: **50% 감소**
- 탐색 시간: **5-10% 감소**

**구현 난이도**: 매우 쉬움

---

### 🌟 방안 5: 스트리밍 처리 (Medium Impact)

**현재 문제**:
```go
// 1단계: 모든 파일 수집
scanFolder(folder, &files)

// 2단계: 처리 시작
processFiles(files)
```

**개선 방안**: 파이프라인 패턴

```go
// 파일 발견 즉시 워커로 전송
fileChan := make(chan ExtendedFileInfo, 100)

// Producer: 파일 탐색
go scanFolderStreaming(folder, fileChan)

// Consumer: 파일 처리
processFilesStreaming(fileChan, resultsChan)
```

**예상 효과**:
- 메모리 사용량: **70% 감소**
- Time to First Result: **대폭 단축**
- 전체 시간: **10-15% 감소**

**구현 난이도**: 중간

---

## 권장 구현 순서

### Phase 1: 빠른 개선 (1-2일)
1. ✅ **방안 4: 조기 필터링** (30분)
   - 즉시 적용 가능
   - 메모리 절약 + 약간의 속도 향상

2. ✅ **방안 2: 버퍼 증가** (1-2시간)
   - `godirwalk` 라이브러리 도입
   - 네트워크 I/O 최적화

**예상 효과**: 전체 시간 **15-25% 감소** (2-3분 → 1.5-2분)

### Phase 2: 중간 개선 (3-5일)
3. ✅ **방안 1: 파일 탐색 병렬화** (1-2일)
   - 디렉토리 레벨 병렬 탐색
   - 진행 상황 표시 개선 필요

4. ✅ **방안 5: 스트리밍 처리** (1-2일)
   - 파이프라인 아키텍처로 전환
   - 메모리 효율성 극대화

**예상 효과**: 전체 시간 **50-60% 감소** (2-3분 → 1분 이내)

### Phase 3: 장기 개선 (1-2주)
5. ✅ **방안 3: 캐시 개선** (5-7일)
   - 증분 스캔 구현
   - 파일 변경 감지 시스템

**예상 효과**: 재스캔 시 **90% 이상 감소** (2-3분 → 수 초)

---

## 성능 목표

### 현재 (Baseline)
- **초기 스캔**: 2-3분 (1,491 파일)
- **재스캔**: 2-3분 (캐시 없음)

### Phase 1 완료 후
- **초기 스캔**: 1.5-2분 (25% 개선)
- **재스캔**: 1.5-2분

### Phase 2 완료 후
- **초기 스캔**: 1분 이내 (60% 개선)
- **재스캔**: 1분 이내

### Phase 3 완료 후 (최종 목표)
- **초기 스캔**: 1분 이내
- **재스캔**: 5-10초 (95% 개선) ⭐

---

## 추가 고려사항

### 네트워크 최적화
- **SMB 멀티채널**: 가능하면 활성화
- **NFS 버전**: NFSv4 사용 권장
- **네트워크 MTU**: 점보 프레임 설정 (9000)

### 시스템 튜닝
- **파일 디스크립터**: `ulimit -n` 증가
- **TCP 버퍼**: `sysctl` 튜닝

### 모니터링
- 각 단계별 시간 측정
- 네트워크 I/O 통계
- Worker 활용률 추적

---

## 조합 전략: 모든 방안 단계적 적용

**중요**: 방안들은 상호배타적이지 않으며, 순차적으로 모두 적용 가능합니다!

### 🎯 Phase 1: 즉시 개선 (오늘)
1. ✅ **방안 4**: 조기 필터링 (30분)
2. ✅ **방안 2**: 버퍼 증가 (1-2시간)
3. ✅ **한국어 우선 설정**: Config에서 locale_priority 읽기 (30분)

**예상 효과**: **15-25% 빠름** (2-3분 → 1.5-2분)

### 🚀 Phase 2: 병렬화 (이번 주)
4. ✅ **방안 1**: 디렉토리 병렬 탐색 (1-2일)
5. ✅ **방안 5**: 스트리밍 처리 (1-2일)

**누적 효과**: **50-60% 빠름** (2-3분 → 1분 이내)

### 🔥 Phase 3: 캐싱 (다음 주)
6. ✅ **방안 3**: 증분 스캔 (5-7일)

**최종 효과**: 재스캔 시 **95% 빠름** (2-3분 → 5-10초)

---

## 다음 작업 (2025-10-21)

### 현재 진행 중
- [x] 한국어 우선 메타데이터 설정 확인
- [ ] 한국어 우선 설정을 Config에서 읽어오도록 개선
- [ ] 방안 4: 조기 필터링 구현
- [ ] 방안 2: 버퍼 증가 (godirwalk 도입)
- [ ] 테스트 및 성능 측정
