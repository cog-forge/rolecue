# Spec: RoleCue dùng một session Better Auth

Trạng thái: kiến trúc được chốt trong trao đổi ngày 2026-10-03; chi tiết triển khai bên dưới là cơ sở review plan. Thay thế thiết kế JWT exchange → Go application session của nhánh spike.

## Mục tiêu và phạm vi

Better Auth chạy trong Next, quản lý danh tính và session duy nhất. Browser gọi Go trực tiếp; Go xác thực session qua Better Auth và tự kiểm tra quyền nghiệp vụ. Một user có thể có nhiều session trên nhiều thiết bị.

Worktree: `Better-Auth-Authentication`, branch `feature/authentication`, base `4e204d6`. Base còn auth JWT/password Go cũ; chưa có hai session. Worktree `feature-better-auth` chỉ là nguồn tham khảo UI, không phải nguồn migration hay server auth mới.

Đợt triển khai này: đăng ký email/password, verification/resend, login, Google/GitHub/Facebook OAuth, logout, recovery, middleware Go, role và lock/unlock. Tái sử dụng giao diện đã duyệt; form register chỉ email/password/confirm, dùng tên tạm cho yêu cầu của Better Auth; hồ sơ và chọn role sau onboarding.

Profile editor, change-password UI, username login và 2FA UI là capability tiếp theo. Schema phải có thiết kế tương thích plugin 2FA đã thống nhất; không bật 2FA cho user khi chưa có đầy đủ flow. Chưa quyết định 2FA có bắt buộc sau OAuth hay không. Không coi đăng ký qua OAuth thành công là đã kiểm chứng 2FA.

## Topology và credential

- Production: Next `https://rolecue.dorriss.com`; Go `https://rolecue-api.dorriss.com`.
- Cookie Better Auth: `Domain=dorriss.com`, `Path=/`, `HttpOnly`, `Secure`, `SameSite=Lax`. Phạm vi chia sẻ các subdomain đã được user chấp nhận.
- Dev: Next `http://localhost:3000`, Go `http://localhost:8080`; không đặt Domain production, không trộn localhost với 127.0.0.1. Port không tạo biên cookie; tránh chạy cùng host/cookie name với spike khi manual test.
- Axios `withCredentials: true`; Go credentialed CORS cho exact frontend origin; request ghi phải kiểm tra Origin kể cả các sibling subdomain có cùng site. Better Auth trustedOrigins là allowlist rõ ràng.
- Go chỉ gọi URL Next cố định từ config server, không lấy upstream từ request; gọi `/api/auth/get-session?disableCookieCache=true`, chuyển các cookie auth cần thiết. HTTP client timeout hữu hạn, không follow redirect, không ghi cookie/token vào log.
- Go xác thực response: session/user liên kết đúng, UUID hợp lệ, session chưa hết hạn; đọc users mới nhất để kiểm tra verified email, role và lock; không tin user ID/role do browser khai.
- Cookie gia hạn/xóa do Better Auth trả phải được Go chuyển qua response tới browser bằng từng Set-Cookie header, giữ nguyên thuộc tính. Kiểm chứng cả việc user chỉ gọi Go, không mở lại Next get-session.
- Session sống 14 ngày, gia hạn theo hoạt động với updateAge 1 ngày như cấu hình Better Auth hiện tại; không phải lifetime tuyệt đối 14 ngày. Cookie cache tắt, chưa dùng Redis hoặc cache xác thực Go.
- Không có JWT exchange, JWKS, Go session token, refresh token Go hay fallback Bearer.

## Mô hình dữ liệu đích

Đề xuất đặt các bảng dùng chung trong `public` để khớp ERD của team. Đây là lựa chọn tổ chức schema của plan; Better Auth config phải mapping đầy đủ tên bảng/cột.

| Bảng | Quyền quản lý và yêu cầu |
|---|---|
| users | User Better Auth và RoleCue dùng chung UUID; name/email/email_verified/image/timestamps; role candidate/recruiter/admin; is_locked/lock_reason/lock_expires_at; two_factor_enabled theo plugin. |
| accounts | Phương thức đăng nhập: PK id UUID, user_id FK users, account_id text của provider, provider_id, scope nullable, tokens/expiry, password hash nullable, timestamps. Không phải accounts nghiệp vụ cũ. |
| sessions | Better Auth quản lý id/token/user_id/expiry/timestamps/IP/user agent; impersonated_by nullable theo Admin plugin. Không tạo app_sessions. |
| verifications | id/identifier/value/expiry/timestamps; không bắt buộc user_id. |
| two_factors | Sinh từ plugin phiên bản khóa: secret/backup_codes/user_id/verified/failed_verification_count/locked_until; quy tắc mã hóa/ẩn dữ liệu theo plugin. |

UUID cho PK/FK tương ứng, account_id vẫn text vì ID provider không nhất thiết là UUID. Snake_case, timestamptz, constraints và indexes đối chiếu SQL generate từ Better Auth 1.7.7. Không tự đoán unique hoặc field plugin. Username/display_username chưa được chốt cho login: không bật username plugin trong batch này; nếu team cần cột dự trữ thì nullable, chưa công bố behavior.

Role và trạng thái khóa là server-managed, không cho signup/updateUser tự khai; default candidate. Admin plugin mapping banned → is_locked, banReason → lock_reason, banExpires → lock_expires_at. Role enum phải tương thích cấu hình plugin (không dùng default role user hoặc chuỗi nhiều role khi DB không hỗ trợ).

Go quản lý dữ liệu nghiệp vụ của users; email/password/verification/2FA được sửa qua Better Auth. Các FK job_descriptions.user_id và interview_sessions.user_id phải tham chiếu users.id. Không redesign JD, interview, transaction, wallet hoặc voice.

## Các flow và kết quả bắt buộc

1. **Register/verify:** Better Auth tạo user + credential account, yêu cầu verification; user chưa verified không dùng API nghiệp vụ. Xác minh xong login/session hợp lệ → dashboard, không exchange. Tên tạm không khóa khả năng cập nhật hồ sơ sau này.
2. **Login/OAuth:** cùng users.id nếu liên kết hợp lệ; auto-link chỉ khi provider xác minh email. Facebook account_not_linked là kết quả từ chối liên kết có thông báo, không tự bật trusted provider để bỏ qua kiểm chứng.
3. **Authorized API:** cookie → Go → Better Auth → users/ownership. Không session/hết hạn/đã revoke: 401; quyền không đủ hoặc user đang locked nhưng còn session: 403; Next timeout/5xx: 503, fail closed. Không redirect login vì upstream tạm lỗi.
4. **Logout:** Better Auth revoke session hiện tại và xóa cookie đúng Domain/Path; frontend clear query cache sau thành công. Lỗi mạng phải báo chưa hoàn tất, không giả logout thành công. Logout-all nếu mở tính năng sẽ revoke mọi session.
5. **Lock/unlock:** Admin plugin là đầu vào quản trị duy nhất cho đổi trạng thái khóa và revoke mọi sessions; giới hạn quyền admin, từ chối self-ban theo chính sách plugin, không expose impersonation ngoài scope. Unlock không hồi sinh session cũ. Khóa có expiry dùng lifecycle plugin; Go kiểm tra trạng thái sau Better Auth validation để tránh hai logic expiry mâu thuẫn. Không hứa hủy request đã qua authorization trước thời điểm lock.
6. **Recovery:** reset link đúng/hết hạn/dùng lại phải xử lý đúng; reset thành công revoke mọi Better Auth session. Xóa nhu cầu gọi Go revoke riêng.
7. **Me:** Better Auth get-session phục vụ auth lifecycle; Go GET /auth/me phục vụ hồ sơ/quyền ứng dụng, cùng một user và một session, không phải hai hệ thống session.

## Stack, cấu trúc, quy ước

Next 16/React 19/Bun, Better Auth 1.7.7, PostgreSQL/Kysely; Go/Gin/pgx; Axios, TanStack Query, React Hook Form/Zod. Reuse Resend templates/cấu hình sau khi dựng env riêng; không copy secret vào docs hoặc source.

- `frontend/src/features/auth/{components,schemas,hooks,utils}`: UI/validation; `frontend/src/lib/auth`: server/client; `frontend/src/lib/email`: email.
- `frontend/src/app/api/auth/[...all]/route.ts`: mount Better Auth trong Next.
- `api/internal/features/auth`: client xác thực session + đọc user; `api/internal/middleware`: enforcement; `api/migration`: lịch sử migration áp dụng thống nhất.
- `frontend/tests/unit`, `frontend/tests/e2e`, Go tests cạnh package; docs trong `docs`, checklist trong `tasks`.

Ví dụ hợp đồng browser (không gắn Bearer):

```ts
const response = await apiClient.get("/auth/me");
// apiClient đã bật withCredentials; user ID từ response của Go.
```

Dùng kebab-case cho file frontend, Go theo gofmt; snake_case chỉ ở DB mapping, không tự đổi key SDK Better Auth. Remote state qua TanStack Query; SDK Better Auth cho endpoint auth, Axios cho Go.

## Commands và kiểm chứng

Từ root worktree:

```sh
cd frontend && bun install --frozen-lockfile
cd frontend && bun run dev
cd frontend && bun run lint && bun run typecheck && bun run test && bun run build
cd frontend && bun run test:e2e --project=chromium
cd api && make dev
cd api && go test ./... && go build ./...
cd api && make test-integration
cd api && make swagger
```

Mỗi dòng bắt đầu từ root, không chạy nối các dòng cd trong cùng shell. Integration test cần Docker; DB dev riêng phải được cấu hình trước `make dev`/migration.

Tests: session invalid/expired/revoked, response upstream lỗi/timeout/malformed, Set-Cookie renewal; CORS/Origin; role/ownership/locked state; logout/reset/ban sau đó dùng cookie cũ; signup escalation; OAuth verified linking; hai subdomain HTTPS và reload. Tắt cache để request mới sau revoke không dùng kết quả xác thực cũ.

## Ranh giới và hoàn tất

- Luôn: database dev riêng; generate/check schema theo package đã khóa; kiểm tra lỗi thật; giữ spike nguyên trạng; tasks không đưa vào commit.
- Cần chốt trước feature tương ứng: username login; TOTP hay OTP và 2FA sau OAuth; phạm vi edit profile/change-password UI. Không chặn dựng auth một session vì các feature này.
- Không: chạy down -v trên DB spike, gộp lại hai loại session, copy JWT bridge, commit secrets, tuyên bố UI port là auth end-to-end đã chạy.
- Cắt auth cũ trong base sau khi luồng mới đạt checkpoint. Không cần di chuyển tài khoản/password/JD test từ spike; dựng data dev mới. Nếu đổi yêu cầu sang giữ dữ liệu, phải bổ sung mapping và migration trước khi áp dụng.
- Hoàn tất khi login/register/OAuth đã cấu hình → Go API chạy bằng duy nhất Better Auth session; lock/logout/reset có hiệu lực; cookie renewal đúng; frontend/backend legacy auth bị gỡ và tài liệu phản ánh trạng thái thật.

## Nguồn đối chiếu

- https://better-auth.com/docs/concepts/database
- https://better-auth.com/docs/concepts/session-management
- https://better-auth.com/docs/concepts/cookies
- https://better-auth.com/docs/plugins/admin
- https://better-auth.com/docs/plugins/2fa
