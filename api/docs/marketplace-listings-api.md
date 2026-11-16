# Avatar Marketplace: seller, admin and catalog API (KAN-363, KAN-365, KAN-367)

Backend contract for the Marketplace UI (AM-05, AM-07, AM-09). Source of truth for field-level detail is the generated
Swagger (`api/docs/swagger.yaml`, tags `marketplace-seller`, `marketplace-catalog`, `admin-marketplace`). Scope: coin-only,
no fee, no cash-out, no refund or restore, no resale or gifting, **no checkout** (AM-11 is a later lane).

All routes use the session cookie. Responses use the usual envelope `{ "data": ... }` or
`{ "error": { "code", "message" } }`, snake_case. Mutating requests need the trusted `Origin` header and a JSON body
(`Content-Type: application/json`, at most 16 KiB, unknown fields rejected).

## Endpoints

| Method and path | Who | Purpose |
|---|---|---|
| `GET /marketplace/eligible-avatars` | Candidate, Recruiter | Own avatars the caller may still list (`avatar_id`, `name`) |
| `POST /marketplace/my-listings` | Candidate, Recruiter | Create a draft from `avatar_id` (201) |
| `GET /marketplace/my-listings?status&limit&offset` | Candidate, Recruiter | My Listings, most recently changed first |
| `GET /marketplace/my-listings/{id}` | Candidate, Recruiter | One of my listings (someone else's is 404) |
| `PUT /marketplace/my-listings/{id}` | Candidate, Recruiter | Edit a draft or rejected listing |
| `POST /marketplace/my-listings/{id}/submit` | Candidate, Recruiter | `draft`/`rejected` to `pending` |
| `POST /marketplace/my-listings/{id}/delist` | Candidate, Recruiter (owner) | `active` to `delisted` |
| `GET /marketplace/listings?q&sort&limit&offset` | Candidate, Recruiter | Catalog of active listings |
| `GET /marketplace/listings/{id}` | Candidate, Recruiter | Catalog detail |
| `GET /marketplace/listings/{id}/preview` | Candidate, Recruiter | 302 to the signed preview image (active listings only) |
| `GET /marketplace/my-listings/{id}/preview` | Candidate, Recruiter (owner) | Same, any status of my listing |
| `GET /admin/marketplace/listings/{id}/preview` | Admin | Same, any status |
| `GET /admin/marketplace/listings?status&limit&offset` | Admin | Queue (default `status=pending`; `all` = no filter) |
| `GET /admin/marketplace/listings/{id}` | Admin | Exact payload under review |
| `POST /admin/marketplace/listings/{id}/approve` | Admin | `pending` to `active` |
| `POST /admin/marketplace/listings/{id}/reject` | Admin | `pending` to `rejected` (reason required) |
| `POST /admin/marketplace/listings/{id}/suspend` | Admin | `active` to `suspended` (reason required) |

Roles: the seller and catalog routes also require a verified, unlocked account that finished onboarding; a Guest, an
unauthenticated caller or an Admin is refused (401/403). Admin routes sit under the existing `/admin` role guard and the
service checks the role again.

## Request bodies

- **Create**: `{ avatar_id, title, description?, default_personality, price_coins, rights_note }`. `seller_id` is the session
  user and is never accepted; the asset is resolved from `avatar_id`.
- **Edit** (`PUT`, full replacement): `{ expected_updated_at, title, description?, default_personality, price_coins, rights_note }`.
- **Submit / Delist / Approve**: `{ expected_updated_at }`.
- **Reject**: `{ expected_updated_at, reason }`. **Suspend**: `{ reason }`.
- `expected_updated_at` is the `updated_at` (RFC 3339, microsecond precision) of the listing as the caller last loaded it.
  Send it back unchanged.

Validation (400 `VALIDATION_ERROR`, never a raw database error): `title` 1..100 characters, `description` 0..2000 (blank is
stored as null), `default_personality` 1..2000, `rights_note` 1..2000, `price_coins` integer 1..1,000,000, `reason` 1..2000.
Text is trimmed.

## Responses

**Seller listing** (`my-listings`): `id, asset_id, status, title, description, default_personality, price_coins, rights_note,
review_note, reviewed_at, preview_url, can_edit, can_submit, can_delist, created_at, updated_at`. `review_note` and
`reviewed_at` are filled only while the status is `rejected` or `suspended` (the reason to show the seller).

**Admin listing**: `id, asset_id, asset_ready, seller{display_name}, seller_id, status, title, description,
default_personality, price_coins, rights_note, reviewed_by, reviewed_at, review_note, preview_url, created_at, updated_at`.

**Catalog item**: `id, title, description, default_personality, price_coins, seller{display_name}, already_owned,
is_own_listing, preview_url, created_at`.

**Pages** are `{ items, pagination: { limit, offset, total } }`.

## State machine

```
draft ----submit----> pending ----approve----> active ----delist (seller)---> delisted
  ^                      |                        |
  |                      +----reject----> rejected +--suspend (admin)--> suspended
  +-- edit (stays draft)        rejected --edit (stays rejected)--> submit --> pending
```

| State | Seller may | Admin may | Visible in catalog |
|---|---|---|---|
| `draft` | edit, submit | none | no |
| `pending` | nothing (payload frozen) | approve, reject | no |
| `active` | delist | suspend | yes (if the asset is `ready`) |
| `rejected` | edit, then submit again (an edit since the rejection is required) | none | no |
| `delisted` | nothing | none | no |
| `suspended` | nothing | suspend again (no-op) | no |

`delisted` and `suspended` are terminal in this release: no relist, no unsuspend, no active edit, no hard delete. `seller_id`
and `asset_id` never change after creation. Illegal moves are 409 `MARKETPLACE_INVALID_TRANSITION`.
Both delist and suspend only stop **new** purchases: buyers' avatars, the asset row and the files are never touched, an
asset is never set to `blocked` by them, and the Admin receives no avatar or coins. There is no individual revoke or refund.

**Idempotency:** delist of a delisted listing and suspend of a suspended listing return 200 with the stored row (nothing
is rewritten, the first reason stays). Approve, reject, submit and edit are not idempotent: a repeat is 409.

## Who may list what

Create succeeds only when the avatar is in the caller's own inventory **and** its asset was created by the caller
(`avatar_assets.created_by`) **and** the asset is `ready`. A bought copy (creator is someone else, or null after erasure)
can never be listed. One listing per asset (409 `MARKETPLACE_LISTING_EXISTS`). Submit re-checks that the seller still owns an
avatar of the asset and that it is still sold by its creator and `ready`.

## Concurrency

Every state change is a single guarded `UPDATE` whose `WHERE` holds the expected status **and** `updated_at = expected_updated_at`
(compare-and-set); a stale request matches no row and writes nothing, so no explicit locks are held.

- Edit/submit/delist with an old `updated_at`: 409 `MARKETPLACE_LISTING_STALE`.
- Review is by exact payload: approve/reject fail with `MARKETPLACE_LISTING_STALE` when the seller changed the listing after the
  Admin loaded it (reject, edit, resubmit gives a new `updated_at`). Approve also requires the asset to be `ready`
  (409 `MARKETPLACE_ASSET_NOT_READY`).
- Order of checks for a missed write: not mine (404) → wrong state (`INVALID_TRANSITION`) → stale → asset conditions.
- Concurrent approve/reject of one payload: exactly one wins, the others get 409. A seller edit of a pending listing never succeeds.
- Reject, approve and suspend set `reviewed_by`, `reviewed_at` and `review_note` (cleared on approve) and move `updated_at`
  to the same instant.

## Catalog rules

Only `status = 'active'` listings whose asset is `ready`; every other status is hidden from list **and** detail (404), whoever asks.
`q` is a case-insensitive "title contains" search, trimmed, at most 100 characters, LIKE wildcards are literal.
`sort` is `newest` (default, `created_at DESC, id DESC`), `price_asc` or `price_desc`, served by the partial catalog indexes.
Pagination is offset based: `limit` 1..50 (default 20), `offset` 0..10000, otherwise 400; `total` is the filtered count.
`already_owned` is true when the caller has an avatar of the listing's asset in their own inventory (never read from
transaction descriptions); a deleted bought copy makes it false again. `is_own_listing` is true for the seller, who also gets
`already_owned = true` while the original avatar exists; the UI should offer no purchase in either case (AM-11 refuses
self-purchase).

**Preview.** `preview_url` is a path relative to the API base URL (`/marketplace/listings/{id}/preview`; the seller and admin
views point at their own preview routes). Use it directly as `<img src>`: the API answers **302** to a signed GET (valid
5 minutes, `Cache-Control: private, max-age=60`) of the key `previews/<asset_id>.webp`, which the server builds from a listing
the caller may see (catalog: active and ready only, otherwise 404). The key is never taken from the client and the model
file is never signed. The API does not check that the object exists, so the UI must show its default image when loading
fails (`onError`). No pipeline writes these files yet; seed them for demo data.

Proposed follow-up tickets for generating previews (not built here):
1. **Seller captures in the browser (preferred).** The listing form loads the seller's own avatar with the same VRM renderer
   as the interview room, lets them pick 1..3 angles/poses, and uploads `.webp` through a new upload endpoint (presigned PUT,
   size and dimension limits, webp magic-byte check). The preview becomes part of the reviewed payload: changing it
   needs a new review, and the Admin sees it next to `rights_note`. No Blender or infrastructure change.
2. Server render with the existing Blender converter after a successful conversion (best effort, failure falls back to the
   default image, adds time to the conversion quota). Needs a check that headless rendering works where the API runs.
3. A lightweight 3D viewer would need a separate reduced-quality preview model; the production VRM must stay unloadable before purchase.

## Privacy

No response contains a model key, `model_path`, a signed model URL or any user's email. Buyers never see `rights_note`,
`review_note`, `reviewed_by` or the seller id; they see the seller display name only. Admins see `rights_note` and the seller id
and display name, no email. A seller sees their own `rights_note` and the reason of a rejection or suspension.

## Error codes

| Code | HTTP | Meaning |
|---|---|---|
| `VALIDATION_ERROR` | 400 | Bad body, bounds, pagination, sort, query length, id |
| `INVALID_TOKEN` | 401 | Not signed in |
| `FORBIDDEN` | 403 | Wrong role, locked or unverified account |
| `MARKETPLACE_LISTING_NOT_FOUND` | 404 | No such listing, not yours, or not active (catalog) |
| `MARKETPLACE_ASSET_NOT_ELIGIBLE` | 422 | Avatar not yours, a bought copy, or no longer sold by its creator |
| `MARKETPLACE_ASSET_NOT_READY` | 409 | The asset is `blocked` |
| `MARKETPLACE_LISTING_EXISTS` | 409 | The asset already has a listing |
| `MARKETPLACE_LISTING_STALE` | 409 | `expected_updated_at` no longer matches; reload |
| `MARKETPLACE_INVALID_TRANSITION` | 409 | The listing's state does not allow it (including repeats) |

## For the checkout lane (AM-11)

Read `avatar_listings.status = 'active'` and the asset `status = 'ready'` **inside the purchase transaction** and lock the
listing row (`FOR SHARE`, or `FOR UPDATE` before a price change) so suspend/delist cannot interleave; the guarded updates
here change `status` and `updated_at`, so a buyer who confirmed a price should also compare `price_coins`. Lock order proposal:
buyer user → buyer inventory → listing → asset (FOR SHARE) → wallets (by id order). Use `already_owned`'s predicate
(an avatar with the same `asset_id` in the buyer's inventory) under the inventory lock, and refuse `seller_id = buyer`.
