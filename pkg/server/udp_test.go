package server

import (
	"encoding/binary"
	"testing"
)

func TestParseBareDNSQuestion(t *testing.T) {
	request := []byte{
		0x12, 0x34, 0x01, 0x00, 0x00, 0x01, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x03, 'w', 'w', 'w', 0x07, 'e', 'x', 'a', 'm', 'p', 'l', 'e',
		0x00, 0x00, 0x01, 0x00, 0x01,
	}
	qname, end, ok := parseBareDNSQuestion(request)
	if !ok || qname != "www.example." || end != len(request) {
		t.Fatalf("parseBareDNSQuestion() = (%q, %d, %v)", qname, end, ok)
	}

	withAdditional := append(append([]byte(nil), request...), 0)
	binary.BigEndian.PutUint16(withAdditional[10:12], 1)
	if _, _, ok := parseBareDNSQuestion(withAdditional); ok {
		t.Fatal("parseBareDNSQuestion accepted a packet with an additional section")
	}
}

func TestMakeRawReject(t *testing.T) {
	request := []byte{
		0xab, 0xcd, 0x01, 0x11, 0x00, 0x01, 0x00, 0x02,
		0x00, 0x03, 0x00, 0x04, 0x01, 'a', 0x00, 0x00, 0x01, 0x00, 0x01,
	}
	respLen := makeRawReject(request, len(request), 3)
	if respLen != len(request) {
		t.Fatalf("makeRawReject length = %d, want %d", respLen, len(request))
	}
	flags := binary.BigEndian.Uint16(request[2:4])
	if flags != 0x8193 {
		t.Fatalf("response flags = %#x, want %#x", flags, uint16(0x8193))
	}
	if got := binary.BigEndian.Uint16(request[6:8]); got != 0 || binary.BigEndian.Uint16(request[8:10]) != 0 || binary.BigEndian.Uint16(request[10:12]) != 0 {
		t.Fatalf("response section counts were not cleared: %x", request[6:12])
	}
}
