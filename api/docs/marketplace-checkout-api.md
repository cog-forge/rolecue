# Avatar Marketplace checkout API (KAN-370, AM-11)

Backend contract for the Marketplace UI (AM-12). Coins only: the buyer pays the **full listing price** to the seller, no
fee, no cash-out, no refund, no restore, no resale or gifting. Built on the asset foundation (migration `000036`), the
listings API (`marketplace-listings-api.md`) and the coin transfer primitive and key contract (`marketplace-coins.md`).
No new table or column: the receipt is the existing `transactions` row.

## Endpoint

`POST /marketplace/listings/{id}/purchase`: session cookie, trusted `Origin`, `Content-Type: application/json`.
Candidate and Recruiter with a verified, unlocked account that finished onboarding (the same rules as the catalog); a Guest,
an unauthenticated caller or an Admin gets 401/403.

| Part | Rule |
|---|---|
| Path `id` | The listing the buyer is looking at. |
| Header `Idempotency-Key` | **Required.** `mkt1:<listing id>:<price>:<nonce>`, see below. |
| Body | `{ "price_coins": 20 }`: the price the buyer saw and confirmed (integer 1..1,000,000). Unknown fields are rejected. |

The buyer is the session user. A seller, buyer, amount or avatar is never accepted from the client.

### Response 200

The same shape for a first purchase and for a replay.

```json
{ "data": {
  "avatar": { "id": "…", "name": "Alex the interviewer", "personality": "calm and precise", "created_at": "…", "updated_at": "…" },
  "transaction": { "id": "…", "amount": 20, "currency": "COIN", "created_at": "…" },
  "balance": 30,
  "replayed": false
} }
```

- `avatar` is the inventory avatar DTO (same fields as `GET /inventory/avatars`): no model key, no URL. It is `null` only on a
  replay after the buyer deleted the avatar (the avatar is never restored). If the buyer deleted it and bought it again with a
  new key, a replay of the old key returns the old receipt with the buyer's current copy of the asset.
- `transaction` is the receipt: on a replay it is the row of the first success (original id, amount, time).
- `balance` is the buyer's coin balance when the call returns.
- `replayed` is `true` when the call changed nothing because the same key had already succeeded.

## Idempotency key

`mkt1:<listing id>:<price>:<nonce>`, at most 117 bytes. The listing id is the lowercase canonical UUID of the path `id`,
the price is the decimal `price_coins` of the body (no sign, no leading zero), the nonce is 16..64 characters of
`A-Z a-z 0-9 _ -`. Example: `mkt1:3f1c2d4e-5a6b-4c7d-8e9f-0a1b2c3d4e5f:20:q7Zt3KfA9xLmP2Vd`. The API checks that the key names
exactly this listing and this price.

How the FE builds it:

1. When the buyer opens the confirm dialog, generate the nonce once (for example 22 characters of base64url from
   `crypto.getRandomValues`, or a UUID) and keep it with the dialog state.
2. Build the key from the listing id and the price the dialog shows.
3. **Reuse the same key** for double clicks, network errors, timeouts and "I did not get the response" retries of this one
   purchase, for as long as the confirm dialog (the intent) stays open.
4. **Make a new nonce** for every new intent: each time the confirm dialog is opened again, after a success when the buyer
   deleted the avatar and wants it again, and when the price shown changed (a new price is a new key anyway).

The unique index is on `(buyer, key)`, so a key only ever matches the buyer's own rows. Do not put user ids in the key.

## Behavior

One database transaction, in this lock order (wallet locks always last, see `marketplace-coins.md`): buyer user row (shared),
buyer inventory row, listing row (shared), asset row (shared), then both wallets in ascending user id inside
`ledger.MarketplaceTransfer`. The whole transaction is rerun (same key, at most 3 attempts) on a deadlock or serialization
failure; a rerun is replay-safe. No storage or file IO happens inside it.

1. **Replay by key first.** Under the buyer's inventory lock the buyer's ledger row for the key is read. A row that is the success of this
   intent (seller user, amount equal to the key price, `COIN`, `success`) is returned with `replayed: true`: no debit, no grant,
   no listing check. So a lost response can always be recovered, including after the listing was suspended, delisted or
   deleted, and after the seller was erased. A row that is anything else (another operation or amount) is `COIN_TRANSFER_KEY_CONFLICT`.
2. Otherwise, re-read under the locks, first match wins: listing missing, or draft/pending/rejected (never public): 404
   `MARKETPLACE_LISTING_NOT_FOUND`; delisted or suspended: 409 `MARKETPLACE_LISTING_UNAVAILABLE`; asset not `ready`: 409
   `MARKETPLACE_ASSET_NOT_READY`; buyer is the seller: 409 `MARKETPLACE_OWN_LISTING`; confirmed price differs from the
   listing price: 409 `MARKETPLACE_PRICE_CHANGED`; buyer already owns an avatar of this asset: 409
   `MARKETPLACE_ALREADY_OWNED`; no free inventory slot (same rule as avatar creation): 409 `INVENTORY_FULL`; balance
   too low or no wallet: 422 `INSUFFICIENT_BALANCE`.
3. Then the avatar row `(inventory_id, name = listing title, personality = listing default_personality, asset_id)` is inserted
   for the buyer, the coins move (buyer wallet `-price`, seller wallet `+price`, seller wallet created if missing) and one
   `transactions` row is written (`from` buyer, `to` seller, `COIN`, `success`, the key). The avatar only references the shared
   asset (`asset_id`); the model path lives in `avatar_assets` alone, and the asset and the seller's own avatar are never touched. The debit, the credit, the row and the avatar commit or roll back
   together; a refusal anywhere writes nothing.

Races: purchase against suspend or delist is decided by the listing row lock. Whichever commits first wins: the purchase
either completes fully, or fails with `MARKETPLACE_LISTING_UNAVAILABLE` having written nothing. Two buyers of one listing
both succeed (listings sell copies). Two requests of one buyer for one listing: one succeeds, the others replay (same key) or
get `MARKETPLACE_ALREADY_OWNED` (different keys).

## Error codes

| Code | HTTP | Meaning | Suggested FE reaction |
|---|---|---|---|
| `VALIDATION_ERROR` | 400 | Bad body, price outside 1..1,000,000, bad path id | Fix the request |
| `COIN_TRANSFER_KEY_INVALID` | 400 | Header missing or not a well formed `mkt1` key | Build the key again |
| `INVALID_TOKEN` | 401 | Not signed in | Sign in |
| `FORBIDDEN` | 403 | Wrong role, locked or unverified account | |
| `MARKETPLACE_LISTING_NOT_FOUND` | 404 | No such listing, or not public | Back to the catalog |
| `MARKETPLACE_LISTING_UNAVAILABLE` | 409 | Delisted or suspended since the buyer saw it | Reload the catalog |
| `MARKETPLACE_ASSET_NOT_READY` | 409 | The model is blocked | Reload the catalog |
| `MARKETPLACE_OWN_LISTING` | 409 | The buyer is the seller | Hide the buy button |
| `MARKETPLACE_PRICE_CHANGED` | 409 | `price_coins` is not the listing price | Reload the listing, ask again |
| `MARKETPLACE_ALREADY_OWNED` | 409 | The buyer owns an avatar of this asset | Show "owned" |
| `INVENTORY_FULL` | 409 | No free avatar slot | Offer to delete an avatar or buy a slot |
| `COIN_TRANSFER_KEY_CONFLICT` | 409 | The key names another listing or price than the request, or its stored row is another operation's | New nonce, check the listing and price |
| `COIN_RECIPIENT_BALANCE_LIMIT` | 409 | The seller wallet cannot hold the amount | Try later |
| `INSUFFICIENT_BALANCE` | 422 | Too few coins or no wallet | Offer a top-up |
| `INTERNAL_ERROR` | 500 | Unexpected, or retries exhausted | Retry with the **same** key |

New in this lane: `MARKETPLACE_LISTING_UNAVAILABLE`, `MARKETPLACE_PRICE_CHANGED`, `MARKETPLACE_ALREADY_OWNED`,
`MARKETPLACE_OWN_LISTING` (vi/en messages in `frontend/src/lib/i18n/messages/*/errors.json`). A network error or a 5xx means the
outcome is unknown: retry with the same key and the same body, and the API answers either the original receipt or the failure.

## After the purchase

The buyer owns a normal avatar of the shared asset: it appears in the inventory, can be used in an interview and can be
deleted like any avatar; deleting the last avatar and listing of an asset removes the asset and its file as before. The buyer
**cannot** list a bought copy (only the asset's creator can), resell or gift it, get a refund, or restore it after deleting
it from the old receipt. Buying it again needs a new key and a new payment, and works while the listing is active and the
asset still exists. The catalog shows `already_owned` for it. A purchased avatar does not use the 24-hour GLB conversion
quota (the quota counts assets the user created, not avatar rows). Suspend and delist only stop new purchases.

## Slot purchase

`POST /inventory/purchase` (slot purchase) refuses any `Idempotency-Key` that starts with `mkt1:` (400 `VALIDATION_ERROR`), and
its replay lookup only matches rows paid to the system, so the two namespaces can never answer for each other.

## Notes for later lanes

- AM-12 (FE): everything above; the receipt is `transaction`, the new inventory entry is `avatar`; after a success refresh the
  wallet balance from `balance`.
- AM-13/14/15/16: no table records "who bought what". Ownership is the avatar row; a replay after deletion proves only that
  a payment was made. Account erasure of a seller removes the listing (CASCADE) and a buyer who still holds the avatar keeps it.
  If a later lane needs purchase history or revocation, it needs its own table.
