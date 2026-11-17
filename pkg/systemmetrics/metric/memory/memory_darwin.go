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

package memory

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"syscall"
	"unsafe"

	"github.com/elastic/beats/v7/pkg/opt"
	"github.com/elastic/beats/v7/pkg/systemmetrics/internal/mach"
	"github.com/elastic/beats/v7/pkg/systemmetrics/metric/system/resolve"
)

type xswUsage struct {
	Total, Avail, Used uint64
}

// get is the darwin implementation for fetching Memory data
func get(_ resolve.Resolver) (Memory, error) {
	mem := Memory{}

	var total uint64

	if err := sysctlbyname("hw.memsize", &total); err != nil {
		return Memory{}, fmt.Errorf("error getting memsize: %w", err)
	}
	mem.Total = opt.UintWith(total)

	vmstat, err := mach.HostVMStatistics()
	if err != nil {
		return Memory{}, fmt.Errorf("error getting VM info: %w", err)
	}

	kern := uint64(vmstat.InactiveCount) << 12
	free := uint64(vmstat.FreeCount) << 12

	mem.Free = opt.UintWith(free)
	mem.Used.Bytes = opt.UintWith(total - free)

	mem.Actual.Free = opt.UintWith(free + kern)
	mem.Actual.Used.Bytes = opt.UintWith((total - free) - kern)

	mem.Swap, err = getSwap()
	if err != nil {
		return mem, fmt.Errorf("error getting swap memory: %w", err)
	}

	return mem, nil
}

// Get fetches swap data
func getSwap() (SwapMetrics, error) {
	swUsage := xswUsage{}

	swap := SwapMetrics{}
	if err := sysctlbyname("vm.swapusage", &swUsage); err != nil {
		return swap, fmt.Errorf("error getting swap usage: %w", err)
	}

	swap.Total = opt.UintWith(swUsage.Total)
	swap.Used.Bytes = opt.UintWith(swUsage.Used)
	swap.Free = opt.UintWith(swUsage.Avail)

	return swap, nil
}

// generic Sysctl buffer unmarshalling
func sysctlbyname(name string, data any) (err error) {
	val, err := syscall.Sysctl(name)
	if err != nil {
		return err
	}

	buf := []byte(val)

	switch v := data.(type) {
	case *uint64:
		*v = *(*uint64)(unsafe.Pointer(&buf[0]))
		return nil
	}

	bbuf := bytes.NewBuffer([]byte(val))
	return binary.Read(bbuf, binary.LittleEndian, data)
}
