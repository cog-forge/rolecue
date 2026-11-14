package ledger

import (
	"strconv"
	"strings"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

// MarketplaceKeyNamespace opens every Marketplace purchase key. It is versioned so a later key shape can
// coexist with this one. The inventory slot purchase takes any client string as its key, so the prefix
// alone proves nothing: MarketplaceTransfer also checks the parties of an existing row.
const MarketplaceKeyNamespace = "mkt1"

// MaxKeyLength is the longest idempotency key the API accepts (128 printable ASCII bytes). A Marketplace
// key is at most 117 bytes.
const MaxKeyLength = 128

const (
	minNonceLength = 16
	maxNonceLength = 64
	keySeparator   = ":"
	keySegments    = 4
	maxPriceDigits = 10 // len("2147483647")
)

// MarketplaceKey is a parsed Marketplace purchase key: mkt1:<listing id>:<price>:<nonce>. The listing id
// and the price are the confirmed intent. The nonce is chosen by the client once per purchase intent and
// kept across retries of it; a new purchase of the same listing gets a new nonce.
type MarketplaceKey struct {
	ListingID uuid.UUID
	Price     int32
	Nonce     string
}

// String is the wire form of the key.
func (k MarketplaceKey) String() string {
	return strings.Join([]string{MarketplaceKeyNamespace, k.ListingID.String(), strconv.FormatInt(int64(k.Price), 10), k.Nonce}, keySeparator)
}

// IsMarketplaceKey reports whether key sits in the reserved Marketplace namespace, whether or not the
// rest is well formed. Other operations that accept client keys can use it to refuse the namespace.
func IsMarketplaceKey(key string) bool {
	return strings.HasPrefix(key, MarketplaceKeyNamespace+keySeparator)
}

// NewMarketplaceKey builds a key and runs it through the same validation as a parsed one.
func NewMarketplaceKey(listingID uuid.UUID, price int32, nonce string) (string, error) {
	key := MarketplaceKey{ListingID: listingID, Price: price, Nonce: nonce}.String()
	if _, err := ParseMarketplaceKey(key); err != nil {
		return "", err
	}
	return key, nil
}

// ParseMarketplaceKey checks the key shape exactly: four ':' separated segments, the namespace, a
// lowercase canonical non-nil listing UUID, a canonical decimal price in 1..MaxInt32 and a nonce of
// 16..64 characters from [A-Za-z0-9_-]. Anything else is COIN_TRANSFER_KEY_INVALID. It reads nothing
// from the database.
func ParseMarketplaceKey(key string) (MarketplaceKey, error) {
	if len(key) == 0 || len(key) > MaxKeyLength {
		return MarketplaceKey{}, invalidKey("key must be 1 to 128 bytes")
	}
	parts := strings.Split(key, keySeparator)
	if len(parts) != keySegments || parts[0] != MarketplaceKeyNamespace {
		return MarketplaceKey{}, invalidKey("key is not a Marketplace key")
	}
	listingID, err := uuid.Parse(parts[1])
	if err != nil || listingID == uuid.Nil || listingID.String() != parts[1] {
		return MarketplaceKey{}, invalidKey("key listing id must be a lowercase UUID")
	}
	price, ok := parsePrice(parts[2])
	if !ok {
		return MarketplaceKey{}, invalidKey("key price must be a positive whole number")
	}
	if !validNonce(parts[3]) {
		return MarketplaceKey{}, invalidKey("key nonce must be 16 to 64 characters of A-Z a-z 0-9 _ -")
	}
	return MarketplaceKey{ListingID: listingID, Price: price, Nonce: parts[3]}, nil
}

func invalidKey(message string) error {
	return apperror.New(apperror.CodeCoinTransferKeyInvalid, message)
}

// parsePrice accepts only the canonical form of a positive int32: digits, no sign, no leading zero.
func parsePrice(s string) (int32, bool) {
	if len(s) == 0 || len(s) > maxPriceDigits || s[0] == '0' {
		return 0, false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil || n <= 0 {
		return 0, false
	}
	return int32(n), true
}

func validNonce(s string) bool {
	if len(s) < minNonceLength || len(s) > maxNonceLength {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_', c == '-':
		default:
			return false
		}
	}
	return true
}
