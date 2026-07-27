package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modkernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procReadProcessMemory        = modkernel32.NewProc("ReadProcessMemory")
	procWriteProcessMemory       = modkernel32.NewProc("WriteProcessMemory")
	procCreateToolhelp32Snapshot = modkernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW          = modkernel32.NewProc("Process32FirstW")
	procProcess32NextW           = modkernel32.NewProc("Process32NextW")
	procModule32FirstW           = modkernel32.NewProc("Module32FirstW")
	procModule32NextW            = modkernel32.NewProc("Module32NextW")
)

const (
	TH32CS_SNAPPROCESS        = 0x00000002
	TH32CS_SNAPMODULE         = 0x00000008
	TH32CS_SNAPMODULE32       = 0x00000010
	PROCESS_VM_READ           = 0x0010
	PROCESS_VM_WRITE          = 0x0020
	PROCESS_VM_OPERATION      = 0x0008
	PROCESS_QUERY_INFORMATION = 0x0400
)

type PROCESSENTRY32 struct {
	DwSize              uint32
	CntUsage            uint32
	Th32ProcessID       uint32
	Th32DefaultHeapID   uintptr
	Th32ModuleID        uint32
	CntThreads          uint32
	Th32ParentProcessID uint32
	PcPriClassBase      int32
	DwFlags             uint32
	SzExeFile           [260]uint16
}

type MODULEENTRY32 struct {
	DwSize        uint32
	Th32ModuleID  uint32
	Th32ProcessID uint32
	GlblcntUsage  uint32
	ProccntUsage  uint32
	ModBaseAddr   uintptr
	ModBaseSize   uint32
	HModule       uintptr
	SzModule      [256]uint16
	SzExePath     [260]uint16
}

type ProcessMemoryReader struct {
	Handle syscall.Handle
	PID    uint32
}

func NewProcessMemoryReader(procName string) (*ProcessMemoryReader, error) {
	pid := findProcessId(procName)
	if pid == 0 {
		return nil, fmt.Errorf("process %s not found", procName)
	}
	handle, err := syscall.OpenProcess(PROCESS_VM_READ|PROCESS_VM_WRITE|PROCESS_VM_OPERATION|PROCESS_QUERY_INFORMATION, false, pid)
	if err != nil {
		return nil, err
	}
	return &ProcessMemoryReader{Handle: handle, PID: pid}, nil
}

func (r *ProcessMemoryReader) Close() {
	syscall.CloseHandle(r.Handle)
}

func findProcessId(name string) uint32 {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if snap == ^uintptr(0) {
		return 0
	}
	defer syscall.CloseHandle(syscall.Handle(snap))

	var pe PROCESSENTRY32
	pe.DwSize = uint32(unsafe.Sizeof(pe))

	ret, _, _ := procProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	for ret != 0 {
		exeName := syscall.UTF16ToString(pe.SzExeFile[:])
		if strings.EqualFold(exeName, name) {
			return pe.Th32ProcessID
		}
		ret, _, _ = procProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&pe)))
	}
	return 0
}

func (r *ProcessMemoryReader) GetModuleBase(moduleName string) (uint64, error) {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(uintptr(TH32CS_SNAPMODULE|TH32CS_SNAPMODULE32), uintptr(r.PID))
	if snap == ^uintptr(0) {
		return 0, fmt.Errorf("failed to create module snapshot")
	}
	defer syscall.CloseHandle(syscall.Handle(snap))

	var me MODULEENTRY32
	me.DwSize = uint32(unsafe.Sizeof(me))

	ret, _, _ := procModule32FirstW.Call(snap, uintptr(unsafe.Pointer(&me)))
	for ret != 0 {
		modName := syscall.UTF16ToString(me.SzModule[:])
		if strings.EqualFold(modName, moduleName) {
			return uint64(me.ModBaseAddr), nil
		}
		ret, _, _ = procModule32NextW.Call(snap, uintptr(unsafe.Pointer(&me)))
	}
	return 0, fmt.Errorf("module %s not found", moduleName)
}

func (r *ProcessMemoryReader) ReadBytes(addr uint64, size uint32) []byte {
	buf := make([]byte, size)
	var bytesRead uintptr
	procReadProcessMemory.Call(
		uintptr(r.Handle),
		uintptr(addr),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(size),
		uintptr(unsafe.Pointer(&bytesRead)),
	)
	return buf
}

func (r *ProcessMemoryReader) ReadUint64(addr uint64) uint64 {
	b := r.ReadBytes(addr, 8)
	if len(b) < 8 {
		return 0
	}
	return binary.LittleEndian.Uint64(b)
}

func (r *ProcessMemoryReader) ReadInt32(addr uint64) int32 {
	b := r.ReadBytes(addr, 4)
	if len(b) < 4 {
		return 0
	}
	return int32(binary.LittleEndian.Uint32(b))
}

func (r *ProcessMemoryReader) ReadFloat32(addr uint64) float32 {
	b := r.ReadBytes(addr, 4)
	if len(b) < 4 {
		return 0
	}
	return math.Float32frombits(binary.LittleEndian.Uint32(b))
}

func (r *ProcessMemoryReader) ReadCString(addr uint64, maxLen uint32) string {
	b := r.ReadBytes(addr, maxLen)
	idx := bytes.IndexByte(b, 0)
	if idx >= 0 {
		b = b[:idx]
	}
	return string(b)
}

func (r *ProcessMemoryReader) WriteFloat32(addr uint64, val float32) bool {
	var bytesWritten uintptr
	ret, _, _ := procWriteProcessMemory.Call(
		uintptr(r.Handle),
		uintptr(addr),
		uintptr(unsafe.Pointer(&val)),
		4,
		uintptr(unsafe.Pointer(&bytesWritten)),
	)
	return ret != 0
}

func (r *ProcessMemoryReader) WriteInt32(addr uint64, val int32) bool {
	var bytesWritten uintptr
	ret, _, _ := procWriteProcessMemory.Call(
		uintptr(r.Handle),
		uintptr(addr),
		uintptr(unsafe.Pointer(&val)),
		4,
		uintptr(unsafe.Pointer(&bytesWritten)),
	)
	return ret != 0
}

func isProbableUserPointer(val uint64) bool {
	return val >= 0x10000 && val <= 0x00007FFFFFFFFFFF
}

func isProbablePlayerName(name string) bool {
	if len(name) == 0 || len(name) > 32 {
		return false
	}
	return true
}
