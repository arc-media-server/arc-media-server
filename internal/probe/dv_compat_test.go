package probe

import (
	"encoding/json"
	"testing"
)

func TestDVCompatibilityEvidence(t *testing.T) {
	for _, value := range []int{0, 1, 2, 4, 6} {
		data := []byte(`[{"side_data_type":"DOVI configuration record","dv_profile":8,"rpu_present_flag":1,"dv_bl_signal_compatibility_id":` + string(rune('0'+value)) + `}]`)
		var raw []formatSideData
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatal(err)
		}
		var stream Stream
		applyFormatSideData(&stream, raw)
		if stream.DVBLCompatibilityID == nil || *stream.DVBLCompatibilityID != value {
			t.Fatalf("lost explicit compatibility %d: %+v", value, stream)
		}
	}
}
