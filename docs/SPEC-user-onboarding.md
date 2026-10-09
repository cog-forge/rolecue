# Spec: User Onboarding

Trạng thái: **APPROVED**, người dùng duyệt ngày 2026-10-08.
Module id: user-onboarding.
Nhánh: feature/user-onboarding, rebase trên main sau khi profile và landing được merge (cbb9e11).
Scope nhánh: onboarding lần đầu. KAN-105 và KAN-108 không thuộc scope này.

## Assumptions để review

1. Ứng dụng web Next.js + Go API; Better Auth quản lý cookie/session, DB PostgreSQL hiện có.
2. Chỉ có role tự chọn candidate hoặc recruiter; admin được cấp riêng và không bao giờ xuất hiện như lựa chọn onboarding.
3. Dialog lớn xuất hiện bắt buộc sau lần login đầu tiên, kể cả user tạo bằng OAuth. Account hiện có và account mới đều bắt đầu ở trạng thái pending; không backfill.
4. Onboarding có đúng hai bước bắt buộc: chọn role một lần; nhập metadata theo role đó. Avatar URL được hỏi trong bước metadata cho cả hai role. Email không hỏi lại.
5. Candidate cần tên hiển thị; avatar URL là tùy chọn, để trống thì hệ thống hiện Peek mặc định qua UserAvatar hiện có. Recruiter cần tên hiển thị, tên công ty và website; avatar cũng tùy chọn. Admin được cấp riêng, không qua role chooser.
6. Role đã chọn không đổi được sau khi chọn; nếu rời giữa hai bước, lần sau tiếp tục ở bước metadata.
7. Dùng PostgreSQL container rolecue-postgres đang chạy, DB rolecue trên localhost:5432 khi phát triển. Không tạo DB app mới.

## Objective

Khi user mới có Better Auth session hợp lệ đăng nhập lần đầu, yêu cầu hai bước mandatory: (1) chọn Candidate hoặc Recruiter một lần, (2) nhập metadata cho role đó. Lưu trên cùng user record hiện có. Sau metadata save, UI tải lại auth/profile và điều hướng đúng dashboard; sidebar và quyền truy cập phản ánh role server đã lưu sau refresh.

Không dùng tên mặc định New user để suy luận trạng thái. Trạng thái onboarding riêng phải phân biệt user chưa hoàn tất với user đã hoàn tất, kể cả user mới có tên thật từ OAuth.

### User outcomes

- Candidate hoàn tất form và vào candidate dashboard/sidebar.
- Recruiter hoàn tất form, nhập tên công ty, và vào recruiter dashboard/sidebar.
- Account hiện có cũng bắt đầu onboarding với onboarding_status pending; admin bypass onboarding.
- User chưa hoàn tất không mở được route workspace nghiệp vụ bằng cách bỏ qua UI hoặc gọi API trực tiếp.
- Không có đường nào cho phép onboarding tự gán admin, đổi user khác, hoặc gửi lock/security fields.

## Proposed flow and data contract

1. Better Auth xác thực session/email theo policy hiện tại. Email chưa verified hoặc session không hợp lệ tuân theo SessionGate hiện có.
2. Auth bootstrap trả role, onboarding role-selected state và completion state. Thiếu/sai state không mặc định là complete.
3. User mới thấy step 1 trong shadcn Dialog lớn, có progress hai bước. Chọn role gọi API mới POST /onboarding/role trên Go; server chỉ nhận candidate/recruiter, lấy ID từ session, ghi role và chuyển onboarding_status sang role_selected. Role đã chọn không thể thay đổi qua endpoint này.
4. Auth bootstrap báo role đã chọn nhưng onboarding chưa hoàn tất thì hiển thị step 2, kể cả sau reload/login mới. Workspace vẫn bị chặn. Admin được provision riêng và không vào chooser.
5. Step 2 tiếp tục trong dialog lớn, có form theo role. Tái sử dụng GET/PATCH /profile hiện có cho full_name, image, company_name, company_website. Không thêm endpoint metadata riêng. Form prefill dữ liệu image/company đã có (ví dụ OAuth); avatar bỏ trống dùng Peek từ UserAvatar hiện tại. Website là bắt buộc với recruiter.
6. Preview website thuộc frontend: chuẩn hóa HTTPS, hiển thị domain, favicon tải trực tiếp trong trình duyệt và link mở website. Favicon lỗi thì dùng icon Globe. Không gọi Go API, scrape title/description hay ghi dữ liệu khi preview. Company name/avatar vẫn do recruiter nhập; chỉ submit form mới lưu qua PATCH /profile.
7. Backend PATCH /profile hiện tại được mở rộng để hoàn tất onboarding trong cùng update: khi pending, chỉ đánh dấu hoàn tất nếu metadata bắt buộc cho role đó hợp lệ. User không thể đánh dấu complete bằng client field. Metadata update và chuyển onboarding_status sang completed phải atomic.
8. Auth shell cho workspace chỉ mở khi onboarding complete; successful profile PATCH refetch auth/profile, rồi điều hướng tới dashboard role phù hợp.
9. Retry khi lỗi giữ draft. Retry cùng role trong khi pending là idempotent; chọn role khác sau bước 1 hoặc gọi role selection sau completion bị từ chối; PATCH metadata sau completion tiếp tục được dùng cho chỉnh sửa profile thường theo KAN-33 contract.

Payload API role-selection:

    {
      "role": "recruiter"
    }

Metadata gửi qua profile PATCH hiện có:

    {
      "full_name": "Nam Dang",
      "image": "https://images.example/avatar.png",
      "company_name": "RoleCue",
      "company_website": "https://rolecue.example"
    }

Candidate gửi full_name và image khi có avatar URL; recruiter gửi full_name, company_name, company_website và image nếu có. Avatar null được lưu như null, UI dùng Peek mặc định. Các key company của candidate bị từ chối theo API profile hiện có.

GET /auth/me hiện tại cần trả thêm onboarding_role_selected và onboarding_completed booleans (hoặc state tương đương) để session gate phân biệt hai bước và user đã hoàn tất. Profile response đã có role, full_name, image, company_name, company_website. Profile cache phải refetch sau bước hai.

### Validation đề xuất

- role: enum chính xác candidate | recruiter.
- full_name: trim; bắt buộc, 1–100 Unicode code points.
- image: Avatar URL HTTPS tuyệt đối, hostname hợp lệ, không userinfo, tối đa 2048 ký tự; tùy chọn. Null/blank dùng generated Peek avatar qua UserAvatar hiện có.
- company_name: chỉ recruiter; trim, bắt buộc, 1–200 Unicode code points.
- company_website: chỉ recruiter; bắt buộc khi hoàn tất onboarding; HTTPS absolute URL, hostname hợp lệ, không userinfo, tối đa 2048 ký tự (cùng quy ước URL profile hiện tại).
- Profile PATCH giữ JSON validation hiện có. Role handler bind JSON rồi gọi onboarding service → auth service → auth repository; auth service chỉ cho phép candidate/recruiter. Không ghi nếu metadata bắt buộc không hợp lệ.
- Backend không fetch website hay avatar URL. Frontend hiển thị domain như text và tải favicon trực tiếp trong trình duyệt.

### Migration / existing account behavior

Commit KAN-33 đã có API profile và các cột name, image, company_name, company_website; không cần tạo lại metadata storage/API. Kiểm tra source sau rebase không thấy field, endpoint hoặc migration onboarding nào trong KAN-33.

Migration 7 thêm enum public.onboarding_status với ba giá trị pending, role_selected, completed và cột users.onboarding_status NOT NULL DEFAULT 'pending'. Account mới và hiện có bắt đầu pending; không backfill. Role endpoint chuyển pending sang role_selected, cho retry cùng role khi role_selected và từ chối đổi role. Profile PATCH chuyển role_selected sang completed khi metadata bắt buộc hợp lệ, trong cùng UPDATE. Admin bypass onboarding; API vẫn suy ra hai boolean onboarding_role_selected/onboarding_completed từ status và role. Không cần hai timestamp hay CHECK giữa hai marker.

Preview là UI thuần frontend, không có service/contract preview trong Go. Trình duyệt tải favicon bằng image không qua image optimizer/proxy; favicon không có thì fallback Globe. Preview không xác nhận website đang hoạt động và không cản việc hoàn tất onboarding.

Migration phải chạy tiến về trên DB PostgreSQL hiện có, sửa trực tiếp migration 7 theo yêu cầu, không tạo migration mới, không reset volume. Vì schema change ảnh hưởng DB dùng chung, việc thực thi migration cần được duyệt theo boundary bên dưới trước khi code triển khai.

## Tech Stack

- Next.js 16.3.4, React 19.2.8, TypeScript; Better Auth 1.7.7 client/session.
- Go API và Gin; PostgreSQL 18.6, migration hiện có trong api/migrations.
- Frontend: React Hook Form + Zod cho form; TanStack Query cho server state; Axios apiClient gọi Go trực tiếp.
- Auth bootstrap và profile API hiện có trên nhánh sau rebase. Không thêm session store/token riêng.

## Commands

Từ thư mục repo:

    make -C api run

Trong terminal khác:

    cd frontend && bun run dev --port 3000

Các lệnh kiểm tra dự kiến khi triển khai:

    cd frontend && bun run lint && bun run typecheck && bun run test && bun run build
    cd api && go test ./... && go vet ./... && go build ./...

make -C api run tự gọi db-up/migrate; trước khi dùng trên máy phát triển cần xác nhận compose project tiếp tục gắn container/volume rolecue-postgres hiện tại và env đang nhắm localhost:5432/rolecue. Không chạy reset/rollback migration cho môi trường này.

## Project Structure

- frontend/src/features/auth: session gate/bootstrap; bổ sung trạng thái onboarding và chặn protected workspace.
- frontend/src/features/onboarding: schema, API client và onboarding form/overview.
- frontend/src/app/(auth) hoặc route group phù hợp: route onboarding; không đổi URL workspace hiện có.
- frontend/src/config/routes.ts: đường dẫn onboarding nếu cần.
- api/internal/features/onboarding: role-selection handler/service/repository; metadata vẫn ở profile feature.
- api/internal/handler, api/internal/provider, api/internal/router: route handler, wiring và auth middleware hiện hữu.
- api/migrations: thêm enum onboarding_status, default pending, không backfill account hiện hữu.
- frontend/tests/unit, frontend/tests/e2e: tests theo frontend conventions hiện tại; API tests gần feature/domain tương ứng theo cấu trúc repo.
- docs/: capability map, spec và ghi nhận quyết định.

## Code Style

Theo frontend convention hiện tại: feature giữ form/schema/API của mình; page chỉ compose; route lấy role từ server auth state. Ví dụ contract form:

    const submit = async (values: OnboardingValues) => {
      const profile = await completeOnboarding(values);
      await queryClient.invalidateQueries({ queryKey: ["auth", "me"] });
      await queryClient.invalidateQueries({ queryKey: ["profile"] });
      router.replace(profile.role === "recruiter" ? routes.recruiter.dashboard : routes.dashboard);
    };

Trong code Go, nhận DTO allowlist riêng; lấy user ID từ authenticated context; dùng parameterized SQL và transaction. Không bind request vào model user tổng quát. Tên hàm/type theo conventions đang có của từng package; giữ envelope API hiện hành.

## Testing Strategy

- Backend unit/service: role allowlist; validation/strict payload; unauthenticated/locked/unverified; candidate không ghi company metadata; recruiter metadata; admin không thể tự chọn role; duplicate completion; atomic failure.
- Backend integration với disposable test database: migration mới từ version 6, existing row và user mới đều có onboarding_status pending, request cập nhật đúng một row theo authenticated ID. Không dùng test để truncate/reset DB rolecue dùng phát triển.
- Frontend unit/component: chưa complete hiển thị dialog; role controls/conditional fields; website preview/link/favicon fallback không gọi API; validation/loading/error/retry; dialog không đóng giữa hai bước; complete user không bị hỏi lại; invalidate auth/profile data và chọn destination đúng role.
- E2E trên Next + Go + Better Auth: candidate và recruiter signup/login/email verification (nếu stack test cho phép), OAuth user mới, submit, sidebar/dashboard đúng role sau điều hướng và reload; bypass route/API bị từ chối; account hiện có phải onboard nếu status pending.
- Giữ tests đặt trong frontend/tests/unit và frontend/tests/e2e; chạy frontend lint/typecheck/test/build và Go test/vet/build ở trên. Runtime DB/E2E nên chạy trên test stack riêng hoặc fixtures cleanup, không mutate dữ liệu app ngoài user test cụ thể.
- Không đặt phần trăm coverage giả tạo; coverage bổ sung theo nhánh điều kiện của onboarding/auth gate.

## Boundaries

- **Always:** derive user ID và quyền từ Better Auth session/backend; validate payload server side; reject role admin từ client; preserve existing session model; xử lý user cũ có chủ đích; giữ dữ liệu form khi lỗi; không log cookie/secret.
- **Ask first:** schema/migration chạy trên DB dùng chung; thay đổi API /auth/me contract ngoài field onboarding; thay đổi Better Auth hoặc dependency; đổi policy role sau onboarding; thêm trường candidate ngoài tên hiển thị.
- **Never:** commit secrets; tin role/user ID từ client; cập nhật user khác; xác định trạng thái qua tên New user; expose DB token/credential; reset/drop volume; biến onboarding thành KAN-105/KAN-108.

## Success Criteria

1. Account mới thấy step 1 trước workspace, kể cả OAuth có sẵn tên/avatar; chỉ candidate/recruiter được chọn.
2. Role được lưu một lần bằng API server-authenticated; refresh giữa hai bước đưa user thẳng về step 2.
3. Step 2 hỏi tên; avatar URL không bắt buộc, để trống dùng Peek. Recruiter bắt buộc có tên công ty và website hợp lệ. Dữ liệu metadata đi qua profile PATCH hiện có.
4. Metadata thiếu/sai, request chưa xác thực, role escalation hoặc attempt đổi role bị từ chối; workspace vẫn chặn tới khi PATCH hoàn tất.
5. Sau lưu, sidebar, dashboard, route authorization và profile phản ánh dữ liệu; reload không hỏi lại.
6. User hiện có và user mới đều bắt đầu với status pending; admin bypass onboarding.
7. Onboarding status và metadata thuộc user record hiện có; không tạo user/session thứ hai.
8. Migration tăng tiến từ schema version 6, không xóa business data/volume.

## Dialog visual direction

Dùng shadcn Dialog hiện có, mở dạng modal lớn khoảng 760–900 px trên desktop, glass panel hơi trong với backdrop blur, viền sáng nhẹ và tương phản đủ cho form. Có progress rõ “Choose your role” → “Your details”, form chiếm phần chính của dialog, khoảng cách thoáng; không biến thành popup nhỏ. Trên mobile dùng gần toàn màn hình với nội dung cuộn được. Không đóng bằng Escape/backdrop hoặc nút close khi onboarding bắt buộc chưa xong; vẫn cho logout. Role đã lưu không có nút quay lại để đổi. Giữ keyboard focus trong modal, label/error/status accessible. Các màn pricing/upsell FE tự chọn chỉ hiện sau completion và không cản workspace.

## Open Questions

Không còn câu hỏi blocker. Người dùng xác nhận hai bước onboarding, bỏ backfill account hiện tại, admin bypass role chooser và preview website do frontend quản lý. Backend chỉ chọn role và lưu metadata/completion. Migration 7 chỉ thêm enum/status; không reset DB hay volume.

## Scope exclusion

- KAN-105 admin user list, lock/unlock.
- KAN-108 TOTP, recovery codes.
- Account security/session device management, password/linked account controls.
- User đổi role sau onboarding, role admin self-service, organization/tenant management.
