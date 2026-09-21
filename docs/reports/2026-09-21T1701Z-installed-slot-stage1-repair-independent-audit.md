# Kiểm toán độc lập Stage-1 installed-slot repair

## Phạm vi và ref

- Upstream có thẩm quyền, không thay đổi: `phase-2/upstream-lifecycle` = `2f5dd7b134ad19b9390af9c174095ef94ed09abc`.
- Head đã kiểm toán ban đầu: `e9b68f8b679f6f1b3d97577f484192664f63bb86`.
- Repair head được kiểm toán: `a3060848f7b23edfbfb6c9b780841e7d5d6879eb`.
- Worktree/branch độc lập: `codex/installed-slot-repair-independent-audit`, bắt đầu tại chính repair head. Commit báo cáo này không thuộc implementation ancestry và không được dùng cho delivery.

`git ls-remote` khớp cả ba ref. `merge-base(e9b68f8,a306084)=e9b68f8`, có đúng hai commit và không merge. `merge-base(2f5dd7b,a306084)=2f5dd7b`, có bốn commit (`68f3869`, `e9b68f8`, `3bf8216`, `a306084`) và không merge. GitHub compare khớp trạng thái ahead, không divergence.

## CI và kiểm tra độc lập

- Historical review receipt [35623726784](https://github.com/trungqwe/dual-pool/actions/runs/35623726784): `e9b68f8`, SUCCESS.
- Authoritative repair receipt [35627228700](https://github.com/trungqwe/dual-pool/actions/runs/35627228700): branch `phase-2/installed-slot-audit-repair`, head `a306084`, SUCCESS. Đã kiểm từng bước `source-checks`: format, modules, vet, tests, race, pinned upstream integration, Windows build, Phase 0 regressions và candidate lock validation.
- Local audit worktree mới: `go test ./internal/installedslot ./internal/instance` PASS.
- Focused green: A/B/B2/C, positive pin, no-rebind, strict JSON, version matrix và publication matrix PASS.

Nội dung `CI PENDING` trong report repair là trạng thái lịch sử lúc commit; receipt GitHub ở trên là bằng chứng delivery cuối cùng và không phải failed gate.

## Red/green reproduction

Chỉ trong worktree disposable tại `e9b68f8`, đã chép hai regression test mới, không chép source repair và không commit. OLD fail đúng nguyên nhân:

- `TestProductionPolicyRejectsVersionAliasBeforeVerifier`: `version alias accepted`.
- `TestProductionPolicyRejectsCoherentTamperBeforeVerifier`: bytes chưa pin đi tới executable verifier.
- `TestExistingInstallBootstrapsRegistryBeforeReuseSuccess`: reused slot không có vì registry missing.
- `TestRegistryRemovedAfterConstructionFailsBeforeProcessHandle`: nil registry bypass.

Ở `a306084`, cùng four tests PASS, cùng với `TestExistingInstallCannotReportSuccessForCorruptRegistry`, `TestProductionPolicyAcceptsPinnedIdentity`, `TestProductionPolicyResolveRejectsCoherentTamperingBeforeVerifier`, `TestRegistryRejectsCoherentRebindWithoutChangingDocument`, `TestRegistryMalformedRegisteredDocumentReturnsCorrupt`, `TestRegistryPublicationFaultMatrix` và `TestWindowsVersionReservedExtensionsAndControls`.

## Kết quả theo invariant

| Mục | Kết quả | Bằng chứng độc lập |
| --- | --- | --- |
| A bootstrap legacy reuse | PROVEN_FIXED | `Manager.Install` giữ GLOBAL rồi gọi `RegisterLocked` trước `reused=true`; corrupt registry trả lỗi, không overwrite. Test xác nhận bytes/manifest giữ nguyên và lần reuse hai idempotent. Đây chỉ là bootstrap khi `Install` được gọi, không phải automatic migration mọi startup. |
| B version binding | PROVEN_FIXED | `Registry.New` lấy `expectedVersion` từ lock; `validateManifest` bắt requested, manifest và pin bằng nhau trước verifier. Alias 7.3.8 không gọi verifier. |
| B2 pinned executable hash | PROVEN_FIXED | expected SHA-256 lấy từ lock; manifest hash được so pin trước hash file và trước verifier. Registration và Resolve sau coherent registry rebind đều chặn tamper với verifier call count 0. Positive control xác nhận pin hợp lệ resolve được và verifier thực sự chạy. |
| C nil registry | PROVEN_FIXED | `recordSlot` fail closed ngay khi registry nil hoặc version trống; không còn fallback path. Test xác nhận `recordSlot`/`stopRecord` fail và opener không bị gọi. `activeSlot` cũng fail closed; `Manager.New` đòi state/registry sau options. |
| No-rebind và JSON | PROVEN_FIXED | Identity exe+manifest đổi đồng bộ vẫn `ErrSlotConflict`, registry bytes không đổi. Ba dạng malformed JSON khởi đầu từ entry hợp lệ đều trả chính xác `ErrRegistryCorrupt`. |
| Publication | PROVEN_FIXED ở phạm vi component | 5 fault points x initial/replacement = 10 case: pre-publication giữ missing/old bytes và vA; post-publication reopen thành document hoàn chỉnh, vA/vB resolve được. |
| Process records | RETAINED | Schema-2 vẫn bind PID, StartTime, instance, executable/config hash, port, version và manifest hash. Schema-1 fail closed; Status/Stop dùng recorded version qua registry và tests giữ PID-reuse/active-state protection. |
| Lock topology | PROVEN_FIXED | `Registry.Register` tự lấy GLOBAL. Ba `RegisterLocked` call site đều trong `Manager.Install` hoặc marker recovery được gọi từ Install, sau GLOBAL; không nested lock. |

Đã review cả targeted diff `e9b68f8..a306084` lẫn final tree `2f5dd7b..a306084`, gồm source/test primary files, state/update regression, docs và evidence. Không có dependency, workflow, upstream lock hoặc live/provider behavior mới trong diff.

## Scope và tính chính xác của evidence

Report/evidence repair phân biệt local suite có exclusion với normal Source CI. Chúng cũng sửa claim mutation quá rộng: test/CI có helper subprocess, loopback listener, synthetic WinCred namespace có cleanup và pinned download/help probe; audit này không thấy production updater, provider, product root, user config, OAuth hay deployment mutation.

`writeDocument` tạo candidate bảo vệ, write + `Sync`, strict/ACL validate, `MoveFileEx(MOVEFILE_WRITE_THROUGH)` cho create, `ReplaceFileW` cho replace, rồi ACL + reload. Tài liệu không hứa quá mức: replacement durability, directory metadata persistence, abrupt process kill, OS crash và power loss vẫn chưa được chứng minh. Pathname TOCTOU/không giữ handle qua launch cũng OPEN.

Production policy là trusted one-pin; multi-version/rollback production không hoàn tất. `installedslot.Registry` thỏa interface nhưng Updater -> Registry -> Manager runtime wiring không có trong slice và vẫn OPEN.

## Findings và verdict

Không tìm thấy correctness hoặc security blocker mới cho trusted one-pin component. Các open gate được nêu rõ: automatic legacy migration ngoài `Install`, multi-version rollback policy, updater runtime composition, retained-handle identity, pathname TOCTOU, abrupt-process/OS-crash/power-loss matrix và vulnerability scan.

**PASS_CONTROLLED_FAST_FORWARD_CONSIDERATION**

Mục tiêu tiếp theo duy nhất: delivery-only run kiểm tra lại ref/ancestry rồi fast-forward `phase-2/upstream-lifecycle` từ `2f5dd7b` tới **exact implementation SHA** `a306084` (không dùng commit audit này), sau đó dừng.
