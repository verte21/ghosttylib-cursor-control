//go:build linux

package picker

import (
	"syscall"
	"unsafe"
)

const (
	ioctlTCGETS     = 0x5401
	ioctlTCSETS     = 0x5402
	ioctlTIOCGWINSZ = 0x5413
)

type TermState struct {
	termios syscall.Termios
}

type winsize struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func IsTerminal(fd int) bool {
	var t syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), ioctlTCGETS, uintptr(unsafe.Pointer(&t)), 0, 0, 0)
	return errno == 0
}

func MakeRaw(fd int) (*TermState, error) {
	var old syscall.Termios
	_, _, errno := syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), ioctlTCGETS, uintptr(unsafe.Pointer(&old)), 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}
	raw := old
	raw.Iflag &^= syscall.IGNBRK | syscall.BRKINT | syscall.PARMRK | syscall.ISTRIP | syscall.INLCR | syscall.IGNCR | syscall.ICRNL | syscall.IXON
	raw.Oflag &^= syscall.OPOST
	raw.Lflag &^= syscall.ECHO | syscall.ECHONL | syscall.ICANON | syscall.ISIG | syscall.IEXTEN
	raw.Cflag &^= syscall.CSIZE | syscall.PARENB
	raw.Cflag |= syscall.CS8
	raw.Cc[syscall.VMIN] = 1
	raw.Cc[syscall.VTIME] = 0

	_, _, errno = syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), ioctlTCSETS, uintptr(unsafe.Pointer(&raw)), 0, 0, 0)
	if errno != 0 {
		return nil, errno
	}
	return &TermState{termios: old}, nil
}

func RestoreTerm(fd int, state *TermState) {
	if state == nil {
		return
	}
	_, _, _ = syscall.Syscall6(syscall.SYS_IOCTL, uintptr(fd), ioctlTCSETS, uintptr(unsafe.Pointer(&state.termios)), 0, 0, 0)
}

func GetTermSize(fd int) (cols, rows int) {
	var ws winsize
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), ioctlTIOCGWINSZ, uintptr(unsafe.Pointer(&ws)))
	if errno != 0 || ws.Col == 0 || ws.Row == 0 {
		return 100, 28
	}
	return int(ws.Col), int(ws.Row)
}
