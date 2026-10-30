package installer

import (
	"bufio"
	"bytes"
	"strconv"
	"strings"
)

const (
	MemInfoPath    = "/proc/meminfo"
	maxMemTotalKiB = 1 << 54
)

func ParseMemTotal(data []byte) uint64 {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 || fields[0] != "MemTotal:" {
			continue
		}
		kib, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil || kib > maxMemTotalKiB {
			return 0
		}
		return kib << 10
	}
	return 0
}

func ReadMemTotal(sys System) uint64 {
	data, err := sys.ReadFile(MemInfoPath)
	if err != nil {
		return 0
	}
	return ParseMemTotal(data)
}
