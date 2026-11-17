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

// Package mach wraps the few Mach kernel calls the system metrics need that
// have no syscall equivalent. They are resolved from libSystem at runtime with
// purego, so no cgo is required.
package mach

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	libSystemPath = "/usr/lib/libSystem.B.dylib"

	kernSuccess = 0
	hostVMInfo  = 2 // HOST_VM_INFO flavor
	// hostVMInfoCount is HOST_VM_INFO_COUNT: sizeof(vm_statistics_data_t) / sizeof(integer_t).
	hostVMInfoCount = 15
)

// VMStatistics holds the page counts from the leading fields of
// vm_statistics_data_t.
type VMStatistics struct {
	FreeCount     uint32
	ActiveCount   uint32
	InactiveCount uint32
	WireCount     uint32
}

// vmStatisticsData mirrors vm_statistics_data_t (60 bytes).
type vmStatisticsData struct {
	VMStatistics
	_ [44]byte
}

// Timebase is the ratio that converts Mach absolute-time units to nanoseconds.
type Timebase struct {
	Numer, Denom uint32
}

type machFuncs struct {
	hostSelf       func() uint32
	hostStatistics func(host uint32, flavor int32, out unsafe.Pointer, count *uint32) int32
	timebaseInfo   func(info *Timebase) int32

	host uint32
}

var (
	loadOnce sync.Once
	funcs    machFuncs
	loadErr  error
)

func load() (*machFuncs, error) {
	loadOnce.Do(func() {
		lib, err := purego.Dlopen(libSystemPath, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
		if err != nil {
			loadErr = fmt.Errorf("loading %s: %w", libSystemPath, err)
			return
		}
		purego.RegisterLibFunc(&funcs.hostSelf, lib, "mach_host_self")
		purego.RegisterLibFunc(&funcs.hostStatistics, lib, "host_statistics")
		purego.RegisterLibFunc(&funcs.timebaseInfo, lib, "mach_timebase_info")
		// mach_host_self takes a new port reference on every call; fetch it once.
		funcs.host = funcs.hostSelf()
	})
	return &funcs, loadErr
}

// HostVMStatistics returns the system-wide virtual memory page counts.
func HostVMStatistics() (VMStatistics, error) {
	f, err := load()
	if err != nil {
		return VMStatistics{}, err
	}
	var data vmStatisticsData
	count := uint32(hostVMInfoCount)
	if status := f.hostStatistics(f.host, hostVMInfo, unsafe.Pointer(&data), &count); status != kernSuccess {
		return VMStatistics{}, fmt.Errorf("host_statistics=%d", status)
	}
	return data.VMStatistics, nil
}

// TimebaseInfo returns the Mach absolute-time to nanosecond ratio. The ratio is
// 1/1 on Intel but 125/3 on Apple Silicon. It falls back to 1/1 on error.
func TimebaseInfo() Timebase {
	f, err := load()
	if err != nil {
		return Timebase{Numer: 1, Denom: 1}
	}
	var tb Timebase
	if f.timebaseInfo(&tb) != kernSuccess || tb.Denom == 0 {
		return Timebase{Numer: 1, Denom: 1}
	}
	return tb
}
