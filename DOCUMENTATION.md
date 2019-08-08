This program is designed to determine the capability of the cluster to
identify the schedulable number of replicas/pods if provided with a set of
resources.

It also follows the same kind of logic as is used by the kubernetes
scheduler itself. Since it is build upon the kubernetes-client library, we
have reused some of the function readily available in the client package
itself, while re-writing others to create this package.

Author: Abhishek Tamrakar(abhishek.tamrakar08@gmail.com)

CONSTANTS

const (
	BYTE = 1 << (10 * iota)
	KILOBYTE
	MEGABYTE
	GIGABYTE
	TERABYTE
)

VARIABLES

var (
	VERSION    string
	BUILD_DATE string
)

FUNCTIONS

func Columns(p *tabwriter.Writer, out string)
func IsSpinnable(remainingCpuReq int64, remainingMemoryReq int64, cpuAsk int64, memoryAsk int64, replicaAsk int, podAllocatable int64) (int64, bool, bool)
    Test how many more pods can be spun with same resources given, per node.

func Rows(p *tabwriter.Writer, format string, out ...interface{})
func ToBytes(s string) int64
func ToMegabytes(s string) int64
    Below two functions taken as it is from bytes, with few modifications as per
    our need.


TYPES

type VPrint []string
    vertically print the list

func (s VPrint) String() string
