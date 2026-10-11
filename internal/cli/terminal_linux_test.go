package cli

import (
	"bufio"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"yoinkdl/internal/transfer"
)

func TestProgressTerminalResizing(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	master, slave := testTerminal(t, 40)
	p := newProgress(slave)
	p.Update(transfer.Snapshot{Bytes: 256, Total: 1000, Workers: 2})
	first := make([]byte, len("\r\x1b[2K")+39)
	if _, err := io.ReadFull(master, first); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(first), "\r\x1b[2K") || !strings.HasSuffix(string(first), "...") {
		t.Fatalf("terminal frame = %q", first)
	}
	size := [4]uint16{24, 24}
	terminalIO(t, slave, syscall.TIOCSWINSZ, unsafe.Pointer(&size))
	p.Finish()
	last := make([]byte, len("\r\x1b[2K")+23+2)
	if _, err := io.ReadFull(master, last); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(last), "\r\x1b[2K") || !strings.HasSuffix(string(last), "...\r\n") {
		t.Fatalf("resized final frame = %q", last)
	}
}

func TestProgressTerminalWithoutANSICapability(t *testing.T) {
	for _, term := range []string{"", "dumb"} {
		t.Run(strconv.Quote(term), func(t *testing.T) {
			t.Setenv("TERM", term)
			master, slave := testTerminal(t, 80)
			p := newProgress(slave)
			p.Update(transfer.Snapshot{Bytes: 2, Total: -1, Workers: 1})
			line, err := bufio.NewReader(master).ReadString('\n')
			if err != nil || strings.Contains(line, "\x1b") || !strings.Contains(line, "1 worker") {
				t.Fatalf("plain terminal frame = %q, %v", line, err)
			}
			p.Finish()
		})
	}
}

func testTerminal(t *testing.T, width uint16) (*os.File, *os.File) {
	t.Helper()
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { master.Close() })
	var unlock int32
	terminalIO(t, master, syscall.TIOCSPTLCK, unsafe.Pointer(&unlock))
	var id uint32
	terminalIO(t, master, syscall.TIOCGPTN, unsafe.Pointer(&id))
	slave, err := os.OpenFile("/dev/pts/"+strconv.Itoa(int(id)), os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { slave.Close() })
	size := [4]uint16{24, width}
	terminalIO(t, slave, syscall.TIOCSWINSZ, unsafe.Pointer(&size))
	if err := master.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	return master, slave
}

func terminalIO(t *testing.T, file *os.File, request uintptr, data unsafe.Pointer) {
	t.Helper()
	raw, err := file.SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var errno syscall.Errno
	if err := raw.Control(func(fd uintptr) {
		_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, fd, request, uintptr(data))
	}); err != nil || errno != 0 {
		t.Fatalf("terminal ioctl failed: %v, %v", err, errno)
	}
}
