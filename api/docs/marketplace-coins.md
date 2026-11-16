# Marketplace coin transfer and retry contract (KAN-369, AM-10)

Status: proposed for review. Go API only: no HTTP endpoint, no migration, no new column or table. The
checkout (KAN-370, AM-11) is the only intended caller.

Code: `api/internal/pkg/ledger` (`transfer.go`, `marketplace_key.go`). Coins only: full price, no fee, no
cash-out, no refund, no PayOS.

## 1. API

```go
func MarketplaceTransfer(ctx context.Context, db ledger.DBTX, in MarketplaceTransferInput) (MarketplaceReceipt, error)

type MarketplaceTransferInput struct {
    Key         string    // mkt1:<listing id>:<price>:<nonce>
    ListingID   uuid.UUID // must equal the id inside Key
    Buyer       uuid.UUID
    Seller      uuid.UUID
    Amount      int32     // the listing price read under the caller's lock; must equal the price inside Key
    Description string    // history text, 1..255 characters; never read back
}
type MarketplaceReceipt struct {
    TransactionID uuid.UUID; Buyer, Seller uuid.UUID; Amount int32
    Description string; CreatedAt time.Time
    Replayed bool // false: this call wrote the row. true: the row already existed, nothing was written
}

func ParseMarketplaceKey(key string) (MarketplaceKey, error)
func NewMarketplaceKey(listingID uuid.UUID, price int32, nonce string) (string, error)
func IsMarketplaceKey(key string) bool // prefix "mkt1:", for operations that must refuse the namespace
func IsRetryable(err error) bool       // PostgreSQL 40P01 or 40001 anywhere in the chain
```

`db` is the caller's open transaction (`pgx.Tx`). The function never begins, commits or rolls back, and
logs "applied in transaction": a nil error and a log line are not proof of commit.

One call writes: buyer wallet `-Amount`, seller wallet `+Amount` (seller wallet is created at 0 if
missing), and **one** `transactions` row: `from` = buyer id, `to` = seller id, `amount`, `currency`
`COIN`, `status` `success`, `description`, `idempotency_key` = Key. (The enum value is `success`; the
Brain spec also says `success`. `completed` does not exist.) No `failed` row is ever written: a refusal
returns an error and writes nothing, as `Debit` and `Credit` do.

## 2. Idempotency key

`mkt1:<listing id>:<price>:<nonce>`

| Part | Rule |
|---|---|
| `mkt1` | Namespace and version, lowercase, exact. |
| listing id | Lowercase canonical hyphenated UUID, not nil. |
| price | Canonical decimal of the confirmed price: 1..2147483647, no sign, no leading zero. |
| nonce | 16..64 characters of `A-Z a-z 0-9 _ -`. Chosen by the client once per purchase intent. |

Exactly four `:`-separated segments, at most 117 bytes (limit 128). Example:
`mkt1:3f1c2d4e-5a6b-4c7d-8e9f-0a1b2c3d4e5f:20:q7Zt3KfA9xLmP2Vd`.

The nonce is stable across double clicks, timeouts and retries of one intent. After the buyer deleted the
avatar and wants it again, the client makes a **new nonce**, so a new key, a new intent and a new charge.
The key carries the intent (listing, price), not the buyer or the seller. The unique index is on `("from",
idempotency_key)`, so a key only ever matches the buyer's own rows: another buyer using the same key text
makes a fresh purchase of their own, and cannot replay or probe the first buyer's row. The seller is bound
after the lookup, through the stored `to`. (Open proposal: embed the buyer id in the key so a foreign buyer
is rejected too; it costs about 5 bytes of nonce.) The key is not an ownership registry.

## 3. Order of checks and outcomes

1. Amount > 0, buyer, seller and listing non-nil, buyer != seller, description 1..255 characters:
   else `VALIDATION_ERROR` (400). No database access.
2. Key parses: else `COIN_TRANSFER_KEY_INVALID` (400). This includes keys of any other operation
   (slot purchase, practice, PayOS, system) since none is in the `mkt1` namespace.
3. Key listing id == `ListingID` and key price == `Amount`: else `COIN_TRANSFER_KEY_CONFLICT` (409). Still no
   database access.
4. Create seller wallet if missing, lock both wallets (section 5), read the buyer's row for the key.

| Stored row for (buyer, Key) | Result |
|---|---|
| none | check balance, move coins, insert the row: `Replayed=false` |
| `to`=seller, `amount`=Amount, `COIN`, `success` | **replay**: write no balance and no ledger row, return the stored row with `Replayed=true`, even if the buyer's balance has dropped since |
| anything else (`to`='system' i.e. a slot row, other seller, other amount, other currency or status) | `COIN_TRANSFER_KEY_CONFLICT` (409), nothing written |

5. First success only: buyer wallet missing or balance < Amount: `INSUFFICIENT_BALANCE` (422);
   seller balance + Amount > 2147483647: `COIN_RECIPIENT_BALANCE_LIMIT` (409).
6. A unique violation on the insert (unreachable under the locks, a backstop) is
   `COIN_TRANSFER_KEY_CONFLICT`. The statement fails inside the caller's transaction, which is aborted:
   the caller rolls back.

Concurrent calls with one key and one buyer serialize on the buyer's wallet lock: one writes, the others
read its row after it commits and replay.

No `description` parsing anywhere. The replay decision uses only `from`, `to`, `amount`, `currency`,
`status` and the key.

## 4. Error codes

All are new except the two existing ones. HTTP statuses are in `pkg/response`; vi/en messages are in
`frontend/src/lib/i18n/messages/{vi,en}/errors.json`.

| Code | HTTP | When |
|---|---|---|
| `COIN_TRANSFER_KEY_INVALID` | 400 | Key is empty, malformed, or not in the `mkt1` namespace |
| `COIN_TRANSFER_KEY_CONFLICT` | 409 | Key names another listing/price than the request, or its stored row is another transfer or another operation's |
| `COIN_RECIPIENT_BALANCE_LIMIT` | 409 | Seller wallet would pass 2147483647 |
| `INSUFFICIENT_BALANCE` (existing) | 422 | Buyer has no wallet or too few coins |
| `VALIDATION_ERROR` (existing) | 400 | Zero/negative amount, self-transfer, nil ids, bad description. AM-11 should answer self-purchase with its own checkout code before it gets here |

## 5. Lock order and deadlocks

Rule: **wallet row locks are always the last locks of a transaction**, and two wallets are taken in
ascending user id (byte order), whoever is buyer. Opposite transfers A->B and B->A therefore lock the same
wallet first and queue instead of deadlocking. Existing operations (`ledger.Debit`, `Credit`,
`CreditForOrder`, slot purchase, interview capacity, practice start) hold at most one wallet lock and take
their other locks first (for example the inventory row, then the wallet), so they cannot form a cycle with
this. The seller wallet is created with `INSERT ... ON CONFLICT DO NOTHING` before any `FOR UPDATE`.

Wallet writers checked: `ledger.Debit/Credit/CreditForOrder`, slot purchase, interview capacity and
refund, practice start, PayOS top-up (each locks its own row first, one wallet last), and account erasure
(deletes inventories before wallets, so it waits for an inventory lock before touching a wallet).

AM-11 order (proposal, to align with lane M1's listing and moderation writes): shared user-row lock
(`LockInventoryOwner`), buyer inventory row (the existing slot-purchase order), the listing row, then
`MarketplaceTransfer`; take no user, inventory or listing lock after it. Granting the avatar (an insert
into the buyer's already locked inventory) after the transfer takes no new lock and is fine.

Retry bound: with this order the primitive does not deadlock with itself and, at the default READ COMMITTED
level, has no serialization failures. If a lock the caller took out of order still produces `40P01` or
`40001`, the whole transaction is rolled back (`IsRetryable(err)`); the caller may rerun the **whole
transaction a bounded number of times** (proposal: 3) with the same key (replay-safe), then return a 5xx. The primitive does not
retry: it cannot, the transaction is aborted and owned by the caller.

## 6. Rollback

Every failure, including a later failure in AM-11 (avatar grant, slot, listing recheck), rolls back both
balances, the created seller wallet and the ledger row together, because all writes are in the caller's
transaction. The same key is then usable for the retried purchase. Tests: `transfer_integration_test.go`.

## 7. What AM-11 must do

- Recheck listing active, price, seller, asset, slot and "already owns this asset" **under its own locks**
  before the transfer; self-purchase and already-owned never reach the ledger (no charge, no grant).
- Pass `Amount` = the listing's current price and `Key` from the client's `Idempotency-Key` header. A
  confirmed price that differs from the listing price is a `PRICE_CHANGED` style refusal of AM-11's own,
  before the primitive.
- Call `MarketplaceTransfer` inside the same transaction as the avatar grant. If `Replayed` is true, **do
  not grant again and do not charge**: return the receipt (the original `TransactionID`). A replay after
  the buyer deleted the avatar does not restore it.
- Never use the transaction row or the description to decide ownership.

## 8. Out of scope and limits

- No HTTP endpoint, listings, checkout, avatar or inventory code, fee, refund, restore, cash-out, PayOS.
- The history is description-only text: there is no listing FK on `transactions`, no ownership registry,
  and no way to restore a deleted avatar from it.
- The primitive cannot see whether an earlier transfer still has its avatar; that is intentional.
- A replay still runs the seller-wallet upsert first. If the seller's account was erased in between (user
  row gone), the foreign key fails and the replay returns `INTERNAL_ERROR` instead of the receipt. AM-11
  may treat a vanished seller as a lost purchase it cannot replay; the buyer's coins moved at the first
  success and no row is lost.
- Open point for review (inventory code, not changed here): `PurchaseSlot` accepts any client string as an
  `Idempotency-Key`. A client could send an `mkt1:` key to the slot endpoint. Proposal: reject
  `ledger.IsMarketplaceKey(key)` in the slot handler's key validation, and/or add `"to" = 'system'` to
  `GetSlotPurchaseByKey`. The Marketplace side is already safe: it conflicts on a slot row.
