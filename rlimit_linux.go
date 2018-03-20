package main

import "fmt"

const (
	rLimitCPU        = iota // CPU time in sec
	rLimitFsize             // Maximum filesize
	rLimitData              // max data size
	rLimitStack             // max stack size
	rLimitCore              // max core file size
	rLimitRss               // max resident set size
	rLimitNproc             // max number of processes
	rLimitNofile            // max number of open files
	rLimitMemlock           // max locked-in-memory address space
	rLimitAs                // address space limit
	rLimitLocks             // maximum file locks held
	rLimitSigpending        // max number of pending signals
	rLimitMsgqueue          // maximum bytes in POSIX mqueues
	rLimitNice              // max nice prio allowed to raise to
	rLimitRtprio            // maximum realtime priority
	rLimitRttime            // timeout for RT tasks in us
)

var rlimitMap = map[string]int{
	"RLIMIT_CPU":        rLimitCPU,
	"RLIMIT_FSIZE":      rLimitFsize,
	"RLIMIT_DATA":       rLimitData,
	"RLIMIT_STACK":      rLimitStack,
	"RLIMIT_CORE":       rLimitCore,
	"RLIMIT_RSS":        rLimitRss,
	"RLIMIT_NPROC":      rLimitNproc,
	"RLIMIT_NOFILE":     rLimitNofile,
	"RLIMIT_MEMLOCK":    rLimitMemlock,
	"RLIMIT_AS":         rLimitAs,
	"RLIMIT_LOCKS":      rLimitLocks,
	"RLIMIT_SIGPENDING": rLimitSigpending,
	"RLIMIT_MSGQUEUE":   rLimitMsgqueue,
	"RLIMIT_NICE":       rLimitNice,
	"RLIMIT_RTPRIO":     rLimitRtprio,
	"RLIMIT_RTTIME":     rLimitRttime,
}

func strToRlimit(key string) (int, error) {
	rl, ok := rlimitMap[key]
	if !ok {
		return 0, fmt.Errorf("wrong rlimit value: %s", key)
	}
	return rl, nil
}
