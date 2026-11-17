// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

//go:build darwin

package process

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
)

// The libproc functions are thin wrappers around the proc_info syscall, which
// is called directly so that no cgo is needed and errno is preserved.
// See bsd/sys/proc_info.h in XNU.
const (
	procInfoCallListPIDs  = 1 // PROC_INFO_CALL_LISTPIDS
	procInfoCallPIDInfo   = 2 // PROC_INFO_CALL_PIDINFO
	procInfoCallPIDRusage = 9 // PROC_INFO_CALL_PIDRUSAGE

	procAllPIDs        = 1 // PROC_ALL_PIDS
	procPIDTaskAllInfo = 2 // PROC_PIDTASKALLINFO
	rusageInfoV0       = 0 // RUSAGE_INFO_V0

	// bsd/sys/proc.h process states.
	sIDL   = 1
	sRUN   = 2
	sSLEEP = 3
	sSTOP  = 4
	sZOMB  = 5

	kernProcArgs2 = 49      // KERN_PROCARGS2
	ctlKern       = 1       // CTL_KERN
	argMax        = 1048576 // ARG_MAX
)

// procBSDInfo mirrors struct proc_bsdinfo (136 bytes).
type procBSDInfo struct {
	Flags       uint32
	Status      uint32
	XStatus     uint32
	Pid         uint32
	Ppid        uint32
	UID         uint32
	Gid         uint32
	Ruid        uint32
	Rgid        uint32
	Svuid       uint32
	Svgid       uint32
	_           uint32
	Comm        [16]byte
	Name        [32]byte
	Nfiles      uint32
	Pgid        uint32
	Pjobc       uint32
	ETdev       uint32
	ETpgid      uint32
	Nice        int32
	StartTvsec  uint64
	StartTvusec uint64
}

// procTaskInfo mirrors struct proc_taskinfo (96 bytes).
type procTaskInfo struct {
	VirtualSize   uint64
	ResidentSize  uint64
	TotalUser     uint64
	TotalSystem   uint64
	ThreadsUser   uint64
	ThreadsSystem uint64
	Policy        int32
	Faults        int32
	Pageins       int32
	CowFaults     int32
	MessagesSent  int32
	MessagesRecv  int32
	SyscallsMach  int32
	SyscallsUnix  int32
	Csw           int32
	Threadnum     int32
	Numrunning    int32
	Priority      int32
}

// procTaskAllInfo mirrors struct proc_taskallinfo (232 bytes).
type procTaskAllInfo struct {
	BSD  procBSDInfo
	Task procTaskInfo
}

// rusageInfoV0Data mirrors struct rusage_info_v0 (96 bytes).
type rusageInfoV0Data struct {
	UUID             [16]byte
	UserTime         uint64
	SystemTime       uint64
	PkgIdleWkups     uint64
	InterruptWkups   uint64
	Pageins          uint64
	WiredSize        uint64
	ResidentSize     uint64
	PhysFootprint    uint64
	ProcStartAbstime uint64
	ProcExitAbstime  uint64
}

func procInfo(call, pid, flavor int, arg uint64, buf unsafe.Pointer, size int) (int, error) {
	r, _, errno := unix.Syscall6(unix.SYS_PROC_INFO,
		uintptr(call), uintptr(pid), uintptr(flavor), uintptr(arg),
		uintptr(buf), uintptr(size))
	if errno != 0 {
		return 0, errno
	}
	return int(r), nil
}

// listPIDs is proc_listpids(PROC_ALL_PIDS, ...). With a nil buffer it returns
// the buffer size in bytes needed to hold the pid list.
func listPIDs(buf []byte) (int, error) {
	if len(buf) == 0 {
		return procInfo(procInfoCallListPIDs, procAllPIDs, 0, 0, nil, 0)
	}
	return procInfo(procInfoCallListPIDs, procAllPIDs, 0, 0, unsafe.Pointer(&buf[0]), len(buf))
}

// pidTaskAllInfo is proc_pidinfo(pid, PROC_PIDTASKALLINFO, ...).
func pidTaskAllInfo(pid int, info *procTaskAllInfo) error {
	size := int(unsafe.Sizeof(*info))
	n, err := procInfo(procInfoCallPIDInfo, pid, procPIDTaskAllInfo, 0, unsafe.Pointer(info), size)
	if err != nil {
		return err
	}
	if n != size {
		return fmt.Errorf("proc_info returned %d bytes, expected %d", n, size)
	}
	return nil
}

// pidRusageV0 is proc_pid_rusage(pid, RUSAGE_INFO_V0, ...).
func pidRusageV0(pid int, info *rusageInfoV0Data) error {
	_, err := procInfo(procInfoCallPIDRusage, pid, rusageInfoV0, 0, unsafe.Pointer(info), 0)
	return err
}

// cString returns the NUL-terminated string at the start of b.
func cString(b []byte) string {
	for i, c := range b {
		if c == 0 {
			return string(b[:i])
		}
	}
	return string(b)
}
