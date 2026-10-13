# KAN-122 — Avaturn GLB sang VRM

## Mục tiêu

Candidate/Recruiter gửi GLB T2 cuối cùng do Avaturn export. API chạy Blender headless với VRM Add-on, upload VRM lên private S3, sau đó lưu một avatar với `model_path` trỏ tới object key. Conversion chạy đồng bộ trong request. Inventory hiện có tiếp tục cấp presigned URL khi đọc avatar.

## API

`POST /inventory/avatars` nhận multipart gồm:

- `file`: GLB T2 có lip-sync, tối đa theo `AVATAR_MAX_INPUT_BYTES`.
- `metadata`: JSON `{ "name": "...", "personality": "..." }`.

Chỉ Candidate/Recruiter đã xác thực và có trusted Origin. API validate input, kiểm tra inventory còn chỗ, convert, upload VRM và insert avatar. Trả `201` cùng avatar metadata khi thành công. Không trả object key; consumer dùng `GET /inventory/avatars/{id}` để lấy model access URL hiện có.

Conversion hoặc upload lỗi thì không insert avatar. DB kiểm tra capacity lần cuối trong transaction trước khi insert. Không có charge trong KAN-122; thanh toán generation thuộc KAN-124.

## Converter

- Blender 5.2.x và VRM Add-on 4.10.2; chạy process headless theo từng request, không cần GUI/MCP.
- `qmuntal/gltf` 0.29.0 kiểm tra giới hạn GLB, resource nội bộ, rig Avaturn profile và deformation thực.
- Add-on import/export GLB/VRM; map rig theo tên bone, giữ shape keys, map năm vowel visemes vào VRM 1.0, giữ blink, material và texture.
- VRM được validate trước khi upload. `model_path` lưu opaque S3 key do storage layer trả về.

## Configuration

```dotenv
AVATAR_CONVERSION_ENABLED=false
AVATAR_BLENDER_BINARY=blender
AVATAR_CONVERSION_TIMEOUT=120s
AVATAR_MAX_INPUT_BYTES=52428800
AVATAR_MAX_OUTPUT_BYTES=104857600
AVATAR_VRM_COMMERCIAL_USAGE=
AVATAR_VRM_LICENSE_URL=
```

Enable conversion chỉ trên host đã cài Blender/add-on. Usage metadata và license URL cần chọn theo quyền sử dụng source; không điền policy ngầm. Không còn concurrency setting vì mỗi request chạy đồng bộ trong API process.

## Giới hạn được chấp nhận

- Không có hàng đợi, progress polling, retry/cancel API hay chống tạo trùng bền vững. Client có thể gửi lại request sau timeout và tạo avatar trùng nếu lần trước đã insert nhưng response bị mất.
- S3 và PostgreSQL không cùng transaction. Nếu upload xong nhưng insert DB thất bại, object có thể bị bỏ lại. Storage interface hiện chưa có Delete.
- Chỉ fixture Avaturn T2 owner cung cấp được xác minh; chưa claim mọi body/outfit đều hỗ trợ.
- VRM generation chưa thu coin. Không release paid flow trước khi KAN-124 xử lý debit/receipt.
