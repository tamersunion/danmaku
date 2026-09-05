package domain

import "strings"

const (
	bilibiliXORCode  = int64(23442827791579)
	bilibiliMask     = int64(2251799813685247)
	bilibiliMaxAID   = bilibiliMask + 1
	bilibiliBase     = int64(58)
	bilibiliAlphabet = "FcwAPNKTMug3GV5Lj7EJnHpWsx4tb8haYeviqBz6rkCy12mUSDQX9RdoZf"
)

func AIDToBVID(aid int64) (string, bool) {
	if aid < 1 || aid > bilibiliMask {
		return "", false
	}
	result := []byte("BV1000000000")
	index := len(result) - 1
	value := (bilibiliMaxAID | aid) ^ bilibiliXORCode
	for value > 0 && index >= 3 {
		result[index] = bilibiliAlphabet[value%bilibiliBase]
		value /= bilibiliBase
		index--
	}
	result[3], result[9] = result[9], result[3]
	result[4], result[7] = result[7], result[4]
	return string(result), true
}

func BVIDToAID(bvid string) (int64, bool) {
	bvid = CanonicalBVID(bvid)
	// A few legacy pools were saved without the constant "BV" prefix. Keep
	// the database untouched and normalize only for the deterministic AID
	// calculation performed while building API responses.
	if len(bvid) != 12 || bvid[:3] != "BV1" {
		return 0, false
	}
	value := []byte(bvid)
	value[3], value[9] = value[9], value[3]
	value[4], value[7] = value[7], value[4]
	var encoded int64
	for _, current := range value[3:] {
		index := strings.IndexByte(bilibiliAlphabet, current)
		if index < 0 {
			return 0, false
		}
		encoded = encoded*bilibiliBase + int64(index)
	}
	aid := (encoded & bilibiliMask) ^ bilibiliXORCode
	if aid < 1 || aid > bilibiliMask {
		return 0, false
	}
	canonical, ok := AIDToBVID(aid)
	return aid, ok && canonical == bvid
}

func CanonicalBVID(bvid string) string {
	bvid = strings.TrimSpace(bvid)
	if len(bvid) == 10 && strings.HasPrefix(bvid, "1") {
		return "BV" + bvid
	}
	return bvid
}
