# Báo cáo Phase 2: Khởi tạo product root và key

## Phạm vi

- HEAD bắt đầu: `20934d794f03770f4f779a9309367a3da56ca1ae`.
- Mục tiêu: đóng `P2-ENTRY-ACL-001` và `P2-ENTRY-KEYS-001` bằng một initializer có thể kiểm tra, chạy lặp an toàn và không tiết lộ bí mật.
- Ngoài phạm vi: cấu hình hoặc chạy CLIProxyAPI, tạo listener, truy cập provider auth root, OAuth, refresh token và nội dung credential.

## Thiết kế ACL

- Product root chỉ được lấy từ `dataroot.ResolveCurrent()` ở production.
- Trước khi tạo, xác minh đường dẫn local tuyệt đối, mọi ancestor hiện hữu không phải reparse point và volume hỗ trợ `FILE_PERSISTENT_ACLS`.
- Tạo thư mục bằng `CreateDirectoryW` với `SECURITY_ATTRIBUTES` ngay từ thời điểm tạo; security descriptor có owner là SID của process token hiện tại, DACL protected và chỉ có ACE allow Full Control cho user hiện tại và LocalSystem. ACE áp dụng cho object và child container/object.
- Mọi thư mục con chuẩn (`bin`, `instances`, `config`, `state`, `backups`, `evidence`, `locks`) được tạo theo cùng chính sách và kiểm tra lại bằng `GetNamedSecurityInfoW`.
- Cây đã tồn tại chỉ được chấp nhận khi không có reparse point, không có entry ngoài layout chuẩn, owner đúng, DACL protected và tập ACE allow đúng chính xác. ACL rộng, inherited hoặc policy không đầy đủ đều fail closed; initializer không tự sửa cây hiện hữu.
- Nếu chỉ tạo được một phần thư mục rồi gặp lỗi, lần chạy sau kiểm tra lại phần đã tạo và tiếp tục; không xóa dữ liệu không thuộc quyền sở hữu đã được chứng minh.

## Thiết kế key và phục hồi

- Registry đóng gồm đúng bốn purpose: Codex client, Codex management, Google client và Google management.
- Mỗi key dài đúng 32 byte từ `crypto/rand`; mọi key phải khác nhau. So sánh và kiểm tra chỉ thực hiện trong bộ nhớ, sau đó zero best effort.
- Preflight đọc đủ bốn key trước mọi write. Key sai độ dài hoặc trùng nhau làm toàn bộ lần chạy fail trước khi ghi.
- Khi đã giữ global lock, initializer kiểm tra lại ACL và key rồi chỉ tạo purpose còn thiếu. Key hợp lệ đã có được giữ nguyên.
- Write lỗi giữa chừng để lại tập key hợp lệ đã ghi; lần chạy lại giữ các key đó và bổ sung phần thiếu. Collision từ RNG được retry hữu hạn; hết retry thì fail mà không ghi giá trị trùng.
- Nếu root chưa tồn tại nhưng bất kỳ product key nào đã tồn tại, trạng thái được coi là conflict và không tạo root hoặc ghi key.
- Kết quả và log chỉ chứa trạng thái, số lượng và boolean an toàn; không chứa root, SID, target name hoặc bytes bí mật.

## Kế hoạch mutation thật

1. Hoàn tất implementation, fixture tests và toàn bộ local gates.
2. Commit, push và chờ Source CI PASS.
3. Chỉ sau PASS, chạy integration initializer thật với `DUALPOOL_RUN_REAL_PRODUCT_INIT=1`.
4. Chạy lần hai để chứng minh idempotence; so sánh key trong bộ nhớ rồi zero.
5. Ghi bằng chứng đã sanitize, cập nhật tài liệu/handoff, commit và xác minh CI cho từng mốc.

## Điều kiện dừng

- Dừng ngay khi bất kỳ phase gate, ACL inspection, secret read/write verification, global lock, secret scan, link check, diff check hoặc CI nào thất bại.
- Mọi claim chưa chứng minh được ghi là `UNKNOWN`; không relabel gate.

## Nguồn Microsoft

- [CreateDirectoryW](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createdirectoryw)
- [Security descriptor control](https://learn.microsoft.com/en-us/windows-hardware/drivers/ifs/security-descriptor-control)
- [Security descriptor string format](https://learn.microsoft.com/en-us/windows/win32/secauthz/security-descriptor-string-format)
- [GetTokenInformation](https://learn.microsoft.com/en-us/windows/win32/api/securitybaseapi/nf-securitybaseapi-gettokeninformation)
- [TOKEN_USER](https://learn.microsoft.com/en-us/windows/win32/api/winnt/ns-winnt-token_user)
- [GetNamedSecurityInfoW](https://learn.microsoft.com/en-us/windows/win32/api/aclapi/nf-aclapi-getnamedsecurityinfow)
- [GetVolumeInformationW](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getvolumeinformationw)

## Trạng thái thực thi

- Chưa chạy mutation thật.
- Gate triển khai: OPEN.

## Kết quả implementation trước CI

- Thêm `internal/winacl`: creation-time protected DACL, owner=current token SID, allow-list chính xác current user + LocalSystem, Full Control, kiểm tra persistent ACL và reparse point.
- Thêm `internal/productinit`: production layout chỉ từ `dataroot.ResolveCurrent()`, tám thư mục chuẩn, global lock, bốn key 32 byte độc lập, reconciliation và fail-closed recovery.
- Thêm real integration test bị khóa bởi `DUALPOOL_RUN_REAL_PRODUCT_INIT=1`; chưa mở gate trong commit này.
- Sửa redirect để từ chối authority có explicit port và thêm full repository race step cho Source CI.
- Local PASS: 123 Go test functions trước bổ sung concurrency, full Go suite, `go test -race -count=1 ./...` (lock stress 209.498 s), `go vet`, build, module verify, 54/54 Node tests và `UPSTREAM_LOCK_VALID`.
- Focused concurrency: `go test -race -count=10 ./internal/productinit` PASS.
- Documentation link scan, changed-file secret scan và `git diff --check`: PASS trước stage; scan staged cuối được chạy lại trước commit.
- Product root touched: false. Product credential targets touched: false. CLIProxyAPI/provider state touched: false.
- Implementation commit, push và Source CI: PENDING.
