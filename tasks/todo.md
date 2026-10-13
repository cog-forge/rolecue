# KAN-122 checklist

- [x] Reuse Blender glTF import and pinned VRM Add-on exporter.
- [x] Keep Avaturn bone mapping as a small versioned profile.
- [x] Validate GLB bounds, skeleton and nonzero five-vowel shape keys in Go.
- [x] Run Blender headless with timeout and temporary files.
- [x] Add bounded synchronous multipart `POST /inventory/avatars`.
- [x] Upload VRM to private S3 and store its object key in `avatars.model_path` after conversion.
- [x] Recheck inventory capacity inside the insert transaction.
- [x] Remove async worker/receipt table, migration, SQLC model, polling/retry/cancel routes.
- [x] Roll back local migration 12; local schema is at version 11.
- [x] Keep coin charging out of KAN-122; it belongs to KAN-124.
- [x] Document retry/orphan/variant limits and preserve fixture outside Git.

Fixture conversion and browser loading were verified before simplifying the persistence flow. The new synchronous endpoint has not been integration-tested after this change. See [implementation report](../docs/KAN-122-IMPLEMENTATION.md).
