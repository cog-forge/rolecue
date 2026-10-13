# KAN-122 implementation

Status: synchronous implementation is ready for owner review; no commit created. This revision follows the simpler flow: receive GLB → Blender → private S3 → insert avatar `model_path`.

## Flow

`POST /inventory/avatars` accepts multipart `file` + `metadata` (`name`, `personality`). The handler streams within a bounded size and route deadline. The service validates the Avaturn T2 profile and lip-sync morphs, checks capacity, runs the pinned Blender/VRM exporter, and uploads the resulting VRM. The repository rechecks capacity under the inventory row lock and inserts the avatar. Existing inventory detail returns the presigned asset URL.

There is no generation table, migration, worker, source GLB staging, polling, retry/cancel endpoint, or persistent deduplication. Conversion errors return directly; no avatar row is written. Charging is outside scope and belongs to KAN-124.

## Runtime

- Blender 5.2.2 LTS and VRM Add-on 4.10.2 are installed on the local machine. Backend runs Blender headless and enables the installed add-on in each subprocess; it does not depend on an open GUI or MCP.
- Runtime settings are in `.env.example`: enable flag, executable path, conversion timeout, input/output byte limits, and explicit VRM usage/license metadata.
- Concurrency is naturally bounded by API capacity/deployment and is no longer configured in KAN-122. No worker concurrency setting exists.

## Fixture evidence

Owner fixture: `/tmp/avaturn-t2-test.glb`, SHA-256 `6db9565fefa61e6f29f2340e81caabc1d4a233379eda201bd7db30b2745cad95` (14,742,856 bytes; 54 joints; 12 meshes; 30 images). The real Blender process converted it to `/tmp/kan122-verified.vrm` (~15 MiB). Three.js + three-vrm loaded the VRM 1.0, observed all five vowel expressions across Head/Teeth/Tongue, and blink. Source/output visual comparison retained the fixture appearance.

Artifacts: `/tmp/kan122-verified.vrm`, `/tmp/kan122-verify/vowel-aa.png`, `/tmp/kan122-verify/neutral.png`, `/tmp/kan122-verify/blink.png`.

The earlier async branch version passed DB/S3 worker integration, but that implementation was removed per owner request. The new synchronous endpoint has not been integration-tested after this simplification. No model media or secrets are tracked in Git.

## Tradeoffs

- A request retry after an uncertain client timeout can create a duplicate avatar because no durable idempotency receipt remains.
- S3 upload and DB insert are separate systems. If upload succeeds and DB insert fails, an orphan object may remain; the current storage interface has no Delete method.
- Only the owner-provided body/outfit is verified. Product usage policy and all-variant coverage remain open. Paid generation depends on KAN-124.

The local migration 12 was rolled back (`12/d avatar_generations`); the migration files and SQLC generated table model were removed. Schema version returned to 11.
