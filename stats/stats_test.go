package stats

import (
	"testing"
)

func TestGlobEscape(t *testing.T) {
	tests := [...]struct {
		name, input, expected string
	}{
		{"plain", "mysite_", "mysite_"},
		{"star", "a*b", `a\*b`},
		{"question mark", "a?b", `a\?b`},
		{"brackets", "a[b]", `a\[b\]`},
		{"backslash", `a\b`, `a\\b`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if actual := globEscape(test.input); actual != test.expected {
				t.Errorf("got %q, expected %q", actual, test.expected)
			}
		})
	}
}

func TestKeyPattern(t *testing.T) {
	const defaultPrefix = "drupal.redis.10.3.1..b6tGz-ZPx_mU6Q1E1vyEZn4Ov1rHrbcBgsbhDJR5Sxo"
	tests := [...]struct {
		name, prefix, key  string
		expectedMatch, bin string
		kind               keyKind
	}{
		{"default prefix", "", defaultPrefix + ":render:entity_view:node:1", "drupal.redis.*", "render", binKey},
		{"default prefix, cache tags", "", defaultPrefix + ":cachetags:node_list", "drupal.redis.*", "cachetags", binKey},
		{"default prefix, timestamp", "", defaultPrefix + ":last_write_timestamp_cache_bootstrap", "drupal.redis.*", "cache_bootstrap", timestampKey},
		{"default prefix, unknown format", "", "drupal.redis.10:no_bin", "drupal.redis.*", "", unknownKey},
		{"default prefix, dots are literal", "", "drupalXredisX1:render:x", "drupal.redis.*", "", unknownKey},
		{"default prefix, not at start", "", "x" + defaultPrefix + ":render:x", "drupal.redis.*", "", unknownKey},
		{"custom prefix", "mysite_", "mysite_:config:system.site", "mysite_:*", "config", binKey},
		{"custom prefix, timestamp", "mysite_", "mysite_:last_write_timestamp_cache_config", "mysite_:*", "cache_config", timestampKey},
		{"custom prefix, bin named like a timestamp", "mysite_", "mysite_:last_write_timestamp_x:cid", "mysite_:*", "last_write_timestamp_x", binKey},
		{"custom prefix, bin without cid", "mysite_", "mysite_:config", "mysite_:*", "", unknownKey},
		{"custom prefix, other site", "mysite_", "othersite_:config:system.site", "mysite_:*", "", unknownKey},
		{"custom prefix, longer prefix", "mysite", "mysite_:config:system.site", "mysite:*", "", unknownKey},
		{"custom prefix, not at start", "mysite_", "xmysite_:config:system.site", "mysite_:*", "", unknownKey},
		{"custom prefix, metacharacters", "my.site*", "my.site*:data:x", `my.site\*:*`, "data", binKey},
		{"custom prefix, dot is literal", "my.site", "myXsite:data:x", "my.site:*", "", unknownKey},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			match, re := keyPattern(test.prefix)
			if match != test.expectedMatch {
				t.Errorf("got match %q, expected %q", match, test.expectedMatch)
			}
			kind, bin := classify(re, test.key)
			if kind != test.kind {
				t.Errorf("got kind %d, expected %d", kind, test.kind)
			}
			if bin != test.bin {
				t.Errorf("got bin %q, expected %q", bin, test.bin)
			}
		})
	}
}

func TestMatchedKeysAndTotalSize(t *testing.T) {
	cs := CacheStats{
		Stats:      map[string]BinStats{"a": {Keys: 2, Size: 10}, "b": {Keys: 3, Size: 20}},
		Timestamps: BinStats{Keys: 1, Size: 5},
	}
	if actual := cs.MatchedKeys(); actual != 6 {
		t.Errorf("got %d matched keys, expected 6", actual)
	}
	if actual := cs.TotalSize(); actual != 35 {
		t.Errorf("got total size %d, expected 35", actual)
	}
}
