package ledger

import (
	"strings"
	"testing"

	"github.com/cog-forge/rolecue/api/pkg/apperror"
	"github.com/google/uuid"
)

const (
	listingA = "3f1c2d4e-5a6b-4c7d-8e9f-0a1b2c3d4e5f"
	nonce16  = "abcdefghij012345"
)

func TestParseMarketplaceKeyAcceptsTheContractShape(t *testing.T) {
	for _, tc := range []struct {
		name  string
		key   string
		price int32
		nonce string
	}{
		{"typical", "mkt1:" + listingA + ":20:" + nonce16, 20, nonce16},
		{"max price", "mkt1:" + listingA + ":2147483647:" + nonce16, 2147483647, nonce16},
		{"hyphen and underscore nonce", "mkt1:" + listingA + ":1:" + "A-b_C-d_E-f_G-h_", 1, "A-b_C-d_E-f_G-h_"},
		{"longest nonce", "mkt1:" + listingA + ":9:" + strings.Repeat("n", 64), 9, strings.Repeat("n", 64)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseMarketplaceKey(tc.key)
			if err != nil {
				t.Fatalf("ParseMarketplaceKey(%q) error = %v", tc.key, err)
			}
			if got.ListingID != uuid.MustParse(listingA) || got.Price != tc.price || got.Nonce != tc.nonce {
				t.Fatalf("parsed = %+v, want listing %s price %d nonce %s", got, listingA, tc.price, tc.nonce)
			}
			if got.String() != tc.key {
				t.Fatalf("String() = %q, want the original key %q", got.String(), tc.key)
			}
			if len(tc.key) > MaxKeyLength {
				t.Fatalf("valid key is %d bytes, over the %d limit", len(tc.key), MaxKeyLength)
			}
		})
	}
}

func TestParseMarketplaceKeyRejectsEverythingElse(t *testing.T) {
	for _, tc := range []struct{ name, key string }{
		{"empty", ""},
		{"no namespace", listingA + ":20:" + nonce16},
		{"slot style key", "a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d"},
		{"other namespace", "slot1:" + listingA + ":20:" + nonce16},
		{"uppercase namespace", "MKT1:" + listingA + ":20:" + nonce16},
		{"older namespace version", "mkt0:" + listingA + ":20:" + nonce16},
		{"missing nonce", "mkt1:" + listingA + ":20"},
		{"extra segment", "mkt1:" + listingA + ":20:" + nonce16 + ":x"},
		{"colon in nonce", "mkt1:" + listingA + ":20:abcdefghij01:345"},
		{"uppercase uuid", "mkt1:" + strings.ToUpper(listingA) + ":20:" + nonce16},
		{"uuid without hyphens", "mkt1:" + strings.ReplaceAll(listingA, "-", "") + ":20:" + nonce16},
		{"braced uuid", "mkt1:{" + listingA + "}:20:" + nonce16},
		{"nil uuid", "mkt1:" + uuid.Nil.String() + ":20:" + nonce16},
		{"not a uuid", "mkt1:not-a-listing:20:" + nonce16},
		{"zero price", "mkt1:" + listingA + ":0:" + nonce16},
		{"negative price", "mkt1:" + listingA + ":-5:" + nonce16},
		{"leading zero price", "mkt1:" + listingA + ":007:" + nonce16},
		{"plus sign price", "mkt1:" + listingA + ":+7:" + nonce16},
		{"decimal price", "mkt1:" + listingA + ":2.5:" + nonce16},
		{"price over int32", "mkt1:" + listingA + ":2147483648:" + nonce16},
		{"price far over int32", "mkt1:" + listingA + ":99999999999999999999:" + nonce16},
		{"empty price", "mkt1:" + listingA + "::" + nonce16},
		{"short nonce", "mkt1:" + listingA + ":20:abcdefghij01234"},
		{"long nonce", "mkt1:" + listingA + ":20:" + strings.Repeat("n", 65)},
		{"nonce with space", "mkt1:" + listingA + ":20:abcdefghij 01234"},
		{"nonce with dot", "mkt1:" + listingA + ":20:abcdefghij.01234"},
		{"nonce with non ASCII", "mkt1:" + listingA + ":20:abcdefghij01234é"},
		{"leading space", " mkt1:" + listingA + ":20:" + nonce16},
		{"trailing newline", "mkt1:" + listingA + ":20:" + nonce16 + "\n"},
		{"over 128 bytes", "mkt1:" + listingA + ":20:" + strings.Repeat("n", 128)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseMarketplaceKey(tc.key); !apperror.IsCode(err, apperror.CodeCoinTransferKeyInvalid) {
				t.Fatalf("ParseMarketplaceKey(%q) error = %v, want %s", tc.key, err, apperror.CodeCoinTransferKeyInvalid)
			}
		})
	}
}

func TestIsMarketplaceKeyReservesTheNamespace(t *testing.T) {
	for key, want := range map[string]bool{
		"mkt1:anything":           true,
		"mkt1:":                   true,
		"mkt1":                    false,
		"mkt10:x":                 false,
		"MKT1:x":                  false,
		"":                        false,
		"user-chosen-slot-key-42": false,
	} {
		if got := IsMarketplaceKey(key); got != want {
			t.Errorf("IsMarketplaceKey(%q) = %v, want %v", key, got, want)
		}
	}
}

func TestNewMarketplaceKeyRoundTripsAndValidates(t *testing.T) {
	listing := uuid.MustParse(listingA)
	key, err := NewMarketplaceKey(listing, 20, nonce16)
	if err != nil {
		t.Fatal(err)
	}
	if want := "mkt1:" + listingA + ":20:" + nonce16; key != want {
		t.Fatalf("key = %q, want %q", key, want)
	}
	for _, tc := range []struct {
		name    string
		listing uuid.UUID
		price   int32
		nonce   string
	}{
		{"nil listing", uuid.Nil, 20, nonce16},
		{"zero price", listing, 0, nonce16},
		{"negative price", listing, -1, nonce16},
		{"short nonce", listing, 20, "short"},
	} {
		if _, err := NewMarketplaceKey(tc.listing, tc.price, tc.nonce); !apperror.IsCode(err, apperror.CodeCoinTransferKeyInvalid) {
			t.Errorf("%s: error = %v, want %s", tc.name, err, apperror.CodeCoinTransferKeyInvalid)
		}
	}
}
