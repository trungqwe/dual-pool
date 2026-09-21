# Kế hoạch kỹ thuật sửa installed-slot registry

## Phạm vi

“Giai đoạn 1” trong run này là bước sửa blocker của slice Phase 2, không phải làm lại roadmap Phase 1 (đã PASS). Chủ sở hữu đã yêu cầu tự hoàn thiện tài liệu, triển khai và báo cáo mà không hỏi lại. Audit gốc: [báo cáo](reports/2026-09-21T1630Z-installed-slot-independent-audit.md).

## Giai đoạn 1 — contract pinned và bootstrap

1. Install reuse phải đăng ký slot dưới GLOBAL, chỉ trả reused=true sau registration thành công. Corrupt/conflicting registry phải giữ nguyên và fail closed.
2. New Registry giữ expected version và expected executable SHA-256 từ validated lock. validateManifest bắt buộc khớp cả hai trước binary verifier. Synthetic fixture hai version không phải production policy.
3. recordSlot không được suy ra trust từ path khi registry nil; lỗi trước mở process handle.
4. Regression qua normal Install với slot cũ/no registry/no marker; registry thật, production constructor/policy, TEMP fixture executable tự báo banner pinned. Chỉ thay hash fixture trong bản lock in-memory hợp lệ; không sửa upstream.lock. Fixture không chạy CLIProxyAPI/listener/provider.
5. Negative production-policy tests cho version alias và coherent executable+manifest tampering, assert verifier chưa được gọi. Positive control pinned hợp lệ.
6. Test no-rebind bằng candidate tự nhất quán; strict registry JSON dựa trên entry hợp lệ; exact missing error; reserved Windows extensions; fault matrix 5 boundaries × initial/replacement; reopen để kiểm tra document và slot cũ.
7. Sửa phạm vi evidence; ghi rõ mutation của legacy full tests và CI. Local run không truy cập WinCred/live gates; các command có exclusion phải ghi nguyên văn, không gọi là full suite.

Files dự kiến: registry.go/test, instance.go/test và regression test riêng, report/evidence, handoff/checklist/traceability. Không thay pin, dependency, workflow, updater lifecycle wiring hoặc upstream ref.

## Acceptance và giới hạn

- A/B/B2/C có regression fail trên behavior cũ và pass sau sửa; lỗi registration không báo thành công, registry/slot cũ không bị viết đè.
- Mỗi commit scoped được kiểm tra formatting, vet, test phù hợp, build, secret scan, docs links và diff check; push bình thường.
- Không tự động reconcile schema-1 process record: operator phải dừng bằng lifecycle cũ hoặc xác minh thủ công process đã kết thúc trước lưu trữ record cũ để phục hồi. Không terminate theo record thiếu binding; không xóa credential/config.
- Atomic publication trong giai đoạn này chỉ chứng minh lỗi injection và reopen; power-loss/OS-crash chưa được kiểm thử. Pathname TOCTOU chưa được loại bỏ.
- Runtime updater với registry thật, policy nhiều release được xác minh riêng, handle-retained validation/publication và complete crash subprocess matrix là công việc tiếp theo, không lẫn trong giai đoạn 1.
- Điều kiện dừng: test bắt buộc fail không giải thích được, cần live/provider/credential mutation, hoặc phải thay upstream policy để làm test pass.

## Trạng thái

Audit: FAIL_REPAIR_REQUIRED tại e9b68f8. Giai đoạn 1: đã sửa A/B/B2/C, regression red/green; xem [báo cáo sửa](reports/2026-09-21T1700Z-installed-slot-stage1-repair.md). Upstream integration và audit độc lập bản sửa: OPEN.
