package instance

import (
	"encoding/binary"
	"net"
	"os"
	"strconv"
	"testing"
	"unsafe"
)

func TestDecodeTCP4UsesABIDerivedLayout(t *testing.T) {
	first, stride := unsafe.Offsetof(tcp4tableLayout{}.rows), unsafe.Sizeof(tcp4row{})
	b := make([]byte, first+stride*2)
	binary.LittleEndian.PutUint32(b[:4], 2)
	rows := (*[2]tcp4row)(unsafe.Pointer(&b[first]))
	rows[0] = tcp4row{state: 2, localAddr: binary.LittleEndian.Uint32([]byte{127, 0, 0, 1}), localPort: 0x7d20, pid: 42}
	rows[1] = tcp4row{state: 2, localAddr: 0, localPort: 0x7e20, pid: 99}
	got, err := decodeTCP4(b)
	if err != nil || len(got) != 2 {
		t.Fatalf("decode: %#v %v", got, err)
	}
	if got[0].address != "127.0.0.1" || got[0].port != 8317 || got[0].pid != 42 || got[0].ipv6 {
		t.Fatalf("row0=%+v", got[0])
	}
	if got[1].address != "0.0.0.0" || got[1].port != 8318 || got[1].pid != 99 {
		t.Fatalf("row1=%+v", got[1])
	}
}

func TestTCP4WindowsABI(t *testing.T) {
	l, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	_, p, _ := net.SplitHostPort(l.Addr().String())
	port, _ := strconv.Atoi(p)
	rows, err := tcpListeners()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if !row.ipv6 && row.address == "127.0.0.1" && row.port == port && row.pid == uint32(os.Getpid()) {
			return
		}
	}
	t.Fatal("P2-TCP4-WINDOWS-ABI-001: listener not found")
}

func TestTCP6WindowsABI(t *testing.T) {
	l, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("P2-TCP6-WINDOWS-ABI-001 unavailable: %v", err)
	}
	defer l.Close()
	_, p, _ := net.SplitHostPort(l.Addr().String())
	port, _ := strconv.Atoi(p)
	rows, err := tcpListeners()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		if row.ipv6 && row.port == port && row.pid == uint32(os.Getpid()) {
			return
		}
	}
	t.Fatal("P2-TCP6-WINDOWS-ABI-001: listener not found")
}

func TestDecodeTCP6UsesABIDerivedLayoutAndRejectsTruncation(t *testing.T) {
	first, stride := unsafe.Offsetof(tcp6tableLayout{}.rows), unsafe.Sizeof(tcp6row{})
	b := make([]byte, first+stride)
	binary.LittleEndian.PutUint32(b[:4], 1)
	row := (*tcp6row)(unsafe.Pointer(&b[first]))
	row.state = 2
	row.localAddr[15] = 1
	row.localPort = 0x7d20
	row.pid = 77
	got, err := decodeTCP6(b)
	if err != nil || len(got) != 1 {
		t.Fatalf("decode: %#v %v", got, err)
	}
	if !got[0].ipv6 || got[0].port != 8317 || got[0].pid != 77 {
		t.Fatalf("row=%+v", got[0])
	}
	if _, err = decodeTCP6(b[:first]); err == nil {
		t.Fatal("truncated IPv6 table accepted")
	}
}

func TestDecodeTCPRejectsMalformedCounts(t *testing.T) {
	for _, decode := range []func([]byte) ([]listener, error){decodeTCP4, decodeTCP6} {
		if _, err := decode([]byte{2, 0, 0, 0}); err == nil {
			t.Fatal("malformed count accepted")
		}
	}
}
