# KAN-122 implementation plan

Status: simplified synchronous flow is ready for owner review; no commit.

1. Validate Avaturn T2 GLB and profile using `qmuntal/gltf`.
2. Convert through Blender 5.2 + VRM Add-on 4.10.2 in a bounded headless process.
3. Add `POST /inventory/avatars`: stream GLB, validate metadata, check capacity, convert, upload VRM, insert avatar/model_path.
4. Recheck capacity under an inventory lock when inserting the avatar.
5. Keep charging out of scope (KAN-124); document synchronous request and S3/DB tradeoffs.

There is no durable job/receipt table, queue, worker, poll, retry, or cancel flow. See [spec](../docs/KAN-122-GLB-TO-VRM-SPEC.md) and [implementation evidence](../docs/KAN-122-IMPLEMENTATION.md).
