# Báo cáo sửa installed-slot — giai đoạn 1

## Header

- Repository: trungqwe/dual-pool; roadmap Phase 2; giai đoạn sửa blocker 1.
- Start HEAD: `3bf8216` (audit dựa trên `e9b68f8b679f6f1b3d97577f484192664f63bb86`).
- Branch: `phase-2/installed-slot-audit-repair`.
- End HEAD / push / exact Source CI: PENDING tại commit; receipt GitHub sau commit là nguồn kết luận delivery.
- Worktree riêng, không có thay đổi người dùng; nhánh review và upstream không thay đổi.

## Mục tiêu và điều tra

Thực hiện [kế hoạch kỹ thuật](../25-INSTALLED-SLOT-REPAIR-PLAN.md) sau [audit FAIL](2026-09-21T1630Z-installed-slot-independent-audit.md). Không thay pin/dependency/workflow, không wiring updater, không real lifecycle/provider/OAuth/product config.

Red run: `go test -count=1 -run 'TestProductionPolicy|TestExistingInstall|TestRegistryRemovedAfterConstruction' ./internal/installedslot ./internal/instance` exit 1 trước sửa; năm regression thất bại đúng nguyên nhân: alias được chấp nhận, unpinned bytes tới verifier, registry thiếu sau reuse, corrupt registry vẫn báo reuse success, nil registry trả slot. Positive pinned control không thất bại.

## Thay đổi và kết quả

| Contract | Sửa | Bằng chứng |
|---|---|---|
| A bootstrap | Install reuse RegisterLocked trước success; propagate lỗi | TestExistingInstallBootstrapsRegistryBeforeReuseSuccess; TestExistingInstallCannotReportSuccessForCorruptRegistry |
| B version binding | validateManifest so expectedVersion | TestProductionPolicyRejectsVersionAliasBeforeVerifier |
| B2 hash binding | expectedExecutableHash từ lock, kiểm tra trước verifier | TestProductionPolicyRejectsCoherentTamperBeforeVerifier; TestProductionPolicyResolveRejectsCoherentTamperingBeforeVerifier |
| C nil registry | recordSlot trả ErrIdentityMismatch trước mở process | TestRegistryRemovedAfterConstructionFailsBeforeProcessHandle |
| No rebind | Test thay cả exe/manifest, registry bytes giữ nguyên | TestRegistryRejectsCoherentRebindWithoutChangingDocument |
| Strict JSON | Entry hợp lệ + mutation, yêu cầu ErrRegistryCorrupt | TestRegistryMalformedRegisteredDocumentReturnsCorrupt |
| Fault publication | 5 hook × initial/replacement; đọc lại document, giữ vA | TestRegistryPublicationFaultMatrix |

Test bootstrap dùng production New, Install, winacl, Registry và WindowsVerifier với executable là bản sao test binary trong TEMP. TestMain chỉ in banner fixture khi gọi đúng `-h`, không mở listener. Lock được decode từ pin thật với hash artifact fixture thay trong bộ nhớ; upstream.lock không đổi. Điều này chứng minh luồng/policy, không chứng minh release thứ hai hoặc product migration live.

## Verification

- Focused registry/instance: exit 0.
- Registry count=100: exit 0; regression instance count=10: exit 0.
- Vet, module verification, Windows build: exit 0.
- Node regression: 55/55, exit 0; upstream lock validation: exit 0.
- Local repository tests: `go test -count=1 -skip 'TestWindowsCredentialManagerIntegration|TestPinnedUpstreamIntegration|TestReal|TestTCP|TestIPv' ./...`, exit 0. Đây là suite có exclusion, không tuyên bố full local suite.
- Focused race count=10: exit 0, registry 6.381s và instance 147.635s; filter bao gồm production-policy, registry, logical-version, bootstrap, nil-registry, stop-record, active-selection, process-record và strict JSON tests.
- Peer review read-only sáu file source/test: không phát hiện blocker mới; không coi là audit độc lập của toàn bộ release.
- JSON/docs links/secret scan/diff check phải pass trước commit.

## Security và hiệu chỉnh lịch sử

Không truy cập product root/credential production, không real updater/lifecycle/provider/OAuth. Các fixture dùng TEMP, ACL thật và synthetic helper subprocess. Một focused package run ban đầu đã chạy TCP listener fixture của instance; Node/full-component tests cũng có synthetic loopback HTTP. Vì vậy không tuyên bố “không listener/process”.

Local command cuối loại WinCred integration và live gates. CI mặc định vẫn chạy synthetic WinCred namespace (có cleanup), loopback tests, subprocess crash helpers và pinned download/help probe. Đây là phạm vi regression workflow có sẵn, không phải product/provider mutation. Evidence cũ nói không mutation tuyệt đối bị supersede bởi audit và báo cáo này; báo cáo đã push trước đó giữ nguyên lịch sử.

## Acceptance và giới hạn

A/B/B2/C: PASS_COMPONENT sau red/green. No-rebind/JSON/error-fault matrix: PASS_COMPONENT. Không có ADR bị đảo; lựa chọn một pin giữ nguyên policy hiện hữu.

Power-loss/OS-crash và abrupt-process registry crash matrix: UNKNOWN/NOT RUN; error injection không thay thế chúng. Registry vẫn pathname-based, chưa handle-retained; policy nhiều release và runtime Updater dùng Registry thật còn OPEN. Status/Stop tests cũ chỉ chứng minh helper/handle contract, chưa phải public-flow state-switch integration. Dynamic vulnerability scan UNKNOWN vì thiếu govulncheck.

## Rollback và handoff

Chỉ thay source/test/tài liệu trong branch mới; rollback bằng revert commit repair nếu cần, không xóa product state/credential. Schema-1 process record tiếp tục fail closed; hướng dẫn reconcile thủ công nằm trong kế hoạch kỹ thuật.

Mục tiêu tiếp theo duy nhất: kiểm toán độc lập bản sửa installed-slot trước controlled integration. Không tự fast-forward upstream trong run này.
