package domain

import "testing"

func TestBilibiliIDConversions(t *testing.T) {
	tests := []struct {
		aid  int64
		bvid string
	}{
		{111298867365120, "BV1L9Uoa9EUx"},
		{643755790, "BV1bY4y1j7RA"},
		{305988942, "BV1aP411K7it"},
		{1054803170, "BV1mH4y1u7UA"},
		{79671692, "BV1EJ411r7kH"},
	}
	for _, test := range tests {
		bvid, ok := AIDToBVID(test.aid)
		if !ok || bvid != test.bvid {
			t.Fatalf("AIDToBVID(%d) = %q, %v", test.aid, bvid, ok)
		}
		aid, ok := BVIDToAID(test.bvid)
		if !ok || aid != test.aid {
			t.Fatalf("BVIDToAID(%q) = %d, %v", test.bvid, aid, ok)
		}
	}
}

func TestBilibiliIDConversionsRejectInvalidInput(t *testing.T) {
	if _, ok := AIDToBVID(0); ok {
		t.Fatal("zero AID was accepted")
	}
	if _, ok := BVIDToAID("BV1invalid"); ok {
		t.Fatal("invalid BVID was accepted")
	}
}

func TestBVIDToAIDAcceptsLegacyValueWithoutPrefix(t *testing.T) {
	aid, ok := BVIDToAID("1EJ411r7kH")
	if !ok || aid != 79671692 {
		t.Fatalf("BVIDToAID legacy value = %d, %v", aid, ok)
	}
}
