package gt06

import (
	"encoding/binary"
	"math"
	"testing"
)

func buildClientFrame(protocol byte, serial uint16, info []byte) []byte {
	content := []byte{protocol}
	content = append(content, info...)
	content = append(content, byte(serial>>8), byte(serial))

	length := byte(len(content) + 1)
	check := length
	for _, b := range content {
		check ^= b
	}
	out := []byte{0x78, 0x78, length}
	out = append(out, content...)
	out = append(out, check, 0x0D, 0x0A)
	return out
}

func TestParseLoginFrame(t *testing.T) {
	imei := "123456789012345"
	info := []byte{0x01, 0x23, 0x45, 0x67, 0x89, 0x01, 0x23, 0x45}
	raw := buildClientFrame(protocolLogin, 1, info)

	f, n, err := ParseFrame(raw)
	if err != nil {
		t.Fatalf("ParseFrame: %v", err)
	}
	if n != len(raw) {
		t.Fatalf("consumed %d of %d", n, len(raw))
	}
	if f.Protocol != protocolLogin || f.Serial != 1 {
		t.Fatalf("frame = %+v", f)
	}
	got, err := ParseIMEI(f.Info)
	if err != nil {
		t.Fatalf("ParseIMEI: %v", err)
	}
	if got != imei {
		t.Fatalf("imei = %q, want %q", got, imei)
	}
}

func TestParseLocationFrame(t *testing.T) {
	latRaw := int32(math.Round(35.0 * 1800000))
	lonRaw := int32(math.Round(-6.0 * 1800000))

	info := []byte{6, 26, 1, 2, 3, 4, 5, 1}
	lat := make([]byte, 4)
	lon := make([]byte, 4)
	binary.BigEndian.PutUint32(lat, uint32(latRaw))
	binary.BigEndian.PutUint32(lon, uint32(lonRaw))
	info = append(info, lat...)
	info = append(info, lon...)
	info = append(info, 42)         // speed
	info = append(info, 0x00, 0x5A) // course 90, status bits clear

	raw := buildClientFrame(protocolLocation, 7, info)
	f, _, err := ParseFrame(raw)
	if err != nil {
		t.Fatalf("ParseFrame: %v", err)
	}
	loc, err := ParseLocation(f.Info)
	if err != nil {
		t.Fatalf("ParseLocation: %v", err)
	}
	if !loc.HasFix {
		t.Fatal("expected a GPS fix")
	}
	if math.Abs(loc.Lat-35.0) > 0.0001 || math.Abs(loc.Lon+6.0) > 0.0001 {
		t.Fatalf("coords = %.6f,%.6f", loc.Lat, loc.Lon)
	}
	if loc.Speed == nil || *loc.Speed != 42 {
		t.Fatalf("speed = %v", loc.Speed)
	}
	if loc.Heading == nil || *loc.Heading != 90 {
		t.Fatalf("heading = %v", loc.Heading)
	}
	want := "2026-01-02T03:04:05Z"
	if loc.RecordedAt.Format("2006-01-02T15:04:05Z") != want {
		t.Fatalf("time = %s, want %s", loc.RecordedAt, want)
	}
}

func TestParseFrameRejectsBadChecksum(t *testing.T) {
	raw := buildClientFrame(protocolLogin, 1, make([]byte, 8))
	raw[len(raw)-3] ^= 0xFF

	if _, _, err := ParseFrame(raw); err != ErrChecksum {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
}

func TestBuildAckIsValidFrame(t *testing.T) {
	ack := BuildAck(protocolLocation, 0x1234, 0)
	f, n, err := ParseFrame(ack)
	if err != nil {
		t.Fatalf("ParseFrame(ack): %v", err)
	}
	if n != len(ack) {
		t.Fatalf("consumed %d of %d", n, len(ack))
	}
	if f.Protocol != protocolLocation || f.Serial != 0x1234 {
		t.Fatalf("ack frame = %+v", f)
	}
	if len(f.Info) != 1 || f.Info[0] != 0 {
		t.Fatalf("ack status = %v", f.Info)
	}
}
