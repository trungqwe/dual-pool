# Kiểm toán installed-slot registry tại e9b68f8

## Phạm vi và kết luận

- Repository: trungqwe/dual-pool; roadmap: Phase 2.
- Base: `2f5dd7b134ad19b9390af9c174095ef94ed09abc`.
- Head kiểm toán: `e9b68f8b679f6f1b3d97577f484192664f63bb86`.
- Nhánh báo cáo: `codex/installed-slot-audit-stage1`; commit báo cáo: PENDING.
- Verdict: **FAIL_REPAIR_REQUIRED**. Đây là kết luận về head cũ, không phải chứng nhận bản sửa.
- Latest owner instruction cho phép hoàn thiện tài liệu và triển khai giai đoạn sửa đầu tiên sau audit. Giai đoạn đó thực hiện ở nhánh khác; không cập nhật nhánh được kiểm toán hoặc upstream.

## Ancestry và CI

Remote refs kiểm tra bằng `git ls-remote`: upstream-lifecycle đúng base; installed-slot-registry đúng head. Merge base bằng base, ancestor check thành công, đúng 2 commits, không merge. GitHub compare: ahead=2, behind=0, status=ahead.

Source CI [35619497269](https://github.com/trungqwe/dual-pool/actions/runs/35619497269) đúng SHA `68f3869bc3530e538973c3e74f75f1b2b8d81b80`; [35623726784](https://github.com/trungqwe/dual-pool/actions/runs/35623726784) đúng SHA head. Workflow, job và từng step đều success. CI không chứng minh các invariant chưa có test.

## Blocker đã xác minh qua source

Line dưới đây thuộc head kiểm toán, chưa sửa.

| ID | Phân loại | File/function/logic | Tái hiện và invariant | Mục tiêu sửa nhỏ nhất |
|---|---|---|---|---|
| A | PROVEN_BLOCKER | `internal/instance/instance.go:208`, Install reuse branch | Slot pinned hợp lệ theo contract cũ, không registry/marker; validateInstall thành công rồi trả reused=true trước RegisterLocked. Active selection sau đó gặp ErrRegistryMissing. Topology Phase 2 trước registry có thể đi đúng nhánh này; không đọc product root để khẳng định trạng thái máy hiện tại. | RegisterLocked dưới GLOBAL trước khi trả reuse success; lỗi đăng ký phải propagate. |
| B | PROVEN_BLOCKER | `internal/installedslot/registry.go:152,311`, New/validateManifest | expectedVersion được gán nhưng không dùng. Lookup/manifest 7.3.8 cùng pinned bytes, tag/commit/lock 7.3.7 được chấp nhận. | Ràng buộc logical/manifest version với pinned version trước verifier. |
| B2 | PROVEN_BLOCKER | `registry.go:294,303,311`; `upstreamstage/stage.go:400,423` | Đổi cả executable và manifest hash cho khớp, giữ provenance đã sao chép. Registry không so hash với pin; WindowsVerifier chạy executable -h và chỉ kiểm tra banner. | Ràng buộc hash manifest và bytes với executable hash trong lock trước mọi execution; test zero verifier calls khi mismatch. |
| C | PROVEN_BLOCKER tại API boundary | `internal/instance/instance.go:150,580`, WithSlotRegistry/recordSlot | New kiểm tra nil nhưng Option là callable công khai: WithSlotRegistry(nil)(manager) sau New làm mất invariant. recordSlot vẫn trả path/hash từ record. Không tìm thấy caller production hiện tại làm điều này; vì thế không khẳng định bypass đã xảy ra live. | Bỏ fallback, fail closed trước mở termination handle; dùng registry fixture tường minh trong tests. |

## Các mục kiểm toán còn lại

- Đã đọc 8 source/test files thay đổi và 10 tài liệu/evidence; không có thay đổi dependency/pin/workflow. `govulncheck` không có: vulnerability scan UNKNOWN.
- State validator chặn slash, backslash, drive/UNC/device, dot, trailing dot/space, control, reserved names kể cả extension/case. Test hiện tại chưa bao phủ đầy đủ extension/case.
- Start lấy active state, VerifyInstalled rồi Resolve. Hai lần kiểm tra trùng lặp; kết quả thứ hai được dùng và revalidate, không dùng quyền từ object thứ nhất. Residual pathname TOCTOU vẫn có; không có handle-retained launch.
- ProcessRecord v2 có instance/PID/start-time/executable hash/config hash/port/version/manifest hash. Strict JSON chặn unknown, duplicate và trailing document; schema 1 bị từ chối. Config hash được ghi/validate hình dạng, không tự chứng minh nội dung config live.
- Status/Stop production bình thường resolve recorded version và kiểm tra PID/start-time/image/hash; active state không relabel process. Ngoại lệ C phá trust boundary. Test recorded-slot hiện tại chỉ gọi helper, không thay đổi active state hoặc chạy public Status/Stop.
- Schema-1 record fail closed là policy an toàn nhưng cần hướng dẫn vận hành: dừng bằng phiên bản cũ hoặc xác minh thủ công process rồi reconcile record; không tự terminate từ record không bound. Tài liệu head cũ chưa đủ chỉ dẫn.
- Manifest alias dùng chung schema; Manager tạo manifest pinned, registry strict decode. Shared type không tự loại bỏ khác biệt validator: B/B2 là ví dụ.
- Fresh install: validate stage, copy/write candidate, validate, publish final, validate final, register, cleanup marker. Recovery có final thì validate/register trước dọn marker; không publish candidate vào registry. Không khắc phục nhánh reuse không marker.
- Register lấy GLOBAL; các caller RegisterLocked hiện tại trong Install/recovery nằm dưới cùng GLOBAL. API exported yêu cầu caller giữ lock; chưa thấy caller sai. Wiring updater-to-public-Manager chưa có, giữ OPEN.
- Updater compile assertion hợp lệ, previous/candidate verifier failures chặn mutation theo fake tests. Chưa có test runtime dùng Registry thật xuyên Updater; không coi production integration là PASS.
- Policy current-lock áp dụng tag/commit/lock digest cho mọi entry: real previous release không thể resolve khi pin đổi. Chấp nhận như giới hạn component một pin hiện tại; là blocker cho mọi tuyên bố production rollback đa phiên bản.
- Publication: candidate Sync, MoveFileEx WRITE_THROUGH lần đầu, ReplaceFileW flags=0 khi thay thế, hậu kiểm. Candidate strict validation đọc buffer đã serialize, không đọc lại file thực tế. Không post-rename Sync và không power-loss test; không chấp nhận bảo đảm OS-crash/power-loss.
- Fault test chỉ AFTER_CANDIDATE_SYNC ở lần tạo đầu. Bốn boundary còn lại và thay thế document đang tồn tại chưa được chứng minh. Cần reopen old-or-valid-new matrix, không gọi injection lỗi thường là process-kill proof.
- Rebind test chỉ đổi exe nên lỗi hash có thể che mất rebind branch. Corrupt JSON tests dùng slots=[] và chấp nhận mọi error nên permissive parser cũng pass. Missing-registry test bỏ qua lỗi Remove. Các assertion này cần sửa.

## Hiệu chỉnh phạm vi evidence

`slot-validation.json` overclaim version binding/policy proof; ACL fixture chỉ kiểm tra loại file. Không dùng các nhãn PASS_COMPONENT cũ để cho phép integration.

`security-gate.json`, handoff và report cũ nói không process/listener/Credential Manager mutation là sai phạm vi: full tests dùng TCP loopback, crash helper subprocess; TestWindowsCredentialManagerIntegration ghi/xóa synthetic test namespace bằng WinCred thật. Source CI bật pinned download/stage và executable -h. Các hành vi này khác với chạy updater/provider/product production. Không có bằng chứng source/diff cho production mutation; không thể chứng minh phủ định mọi hoạt động của máy chỉ từ Git.

Trong audit này chỉ đọc source, refs và receipt; không chạy CLIProxyAPI, listener, WinCred hoặc product/provider mutation. Báo cáo và plan là artifact mới duy nhất.

## Handoff

Giữ reviewed branch và authoritative upstream nguyên SHA. Mục tiêu tiếp theo duy nhất: **repair installed-slot registry audit blockers**. Bản sửa phải có regression red/green và evidence chính xác; audit này không cho phép fast-forward.
