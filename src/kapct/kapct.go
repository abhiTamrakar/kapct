/*
This program is designed to determine the capability of the cluster
to identify the schedulable number of replicas/pods if provided with
a set of resources.

It also follows the same kind of logic as is used by the kubernetes scheduler
itself. Since it is build upon the kubernetes-client library, we have reused
some of the function readily available in the client package itself, while
re-writing others to create this package.

Author: Abhishek Tamrakar(abhishek.tamrakar08@gmail.com)
*/
// package main runs a set of functions to obtain the clusters currrnt
// capacity and remaining resources to calculate the maximum number of pods
// it can spun successfully.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	k8s "k8s.io/client-go/kubernetes"
	cmd "k8s.io/client-go/tools/clientcmd"
)

var (
	version   string
	buildDate string
)

const (
	// BYTE variable is to prepare a formula to convert inputs to bytes
	BYTE = 1 << (10 * iota)
	// KILOBYTE variable is to prepare a formula to convert inputs to bytes
	KILOBYTE
	// MEGABYTE variable is to prepare a formula to convert inputs to bytes
	MEGABYTE
	// GIGABYTE variable is to prepare a formula to convert inputs to bytes
	GIGABYTE
	// TERABYTE variable is to prepare a formula to convert inputs to bytes
	TERABYTE
)

type specs map[string]interface{}

func main() {
	runtime.GOMAXPROCS(2)
	// decalare all required flag variables
	var kubeconfig *string
	var cpuAsk string
	var memoryAsk string
	var cpuLimitAsk string
	var memoryLimitAsk string
	var replicaAsk int
	var version bool
	var legends bool

	// read the kubeconfig file from program switch, if not availble from environment variable or default location.
	if kubeConfigFile := getKubeConfig(); kubeConfigFile != "" {
		kubeconfig = flag.String("kubeconfig", kubeConfigFile, "(optional) absolute path to the kubeconfig file")
	} else {
		kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	}

	// declare the flags and set teh defaults
	flag.StringVar(&cpuAsk, "cpureq", "100m", "amount of CPU you desire in m(milicores), use only string formatted interger for cores.")
	flag.StringVar(&memoryAsk, "memreq", "1G", "amount of memory you desire in K(KB),M(MB),G(GB),T(TB)")
	flag.StringVar(&cpuLimitAsk, "cpulimit", "100m", "amount of CPU you desire in m(milicores), use only string formatted interger for cores.")
	flag.StringVar(&memoryLimitAsk, "memlimit", "1G", "amount of memory you desire in K(KB),M(MB),G(GB),T(TB)")
	flag.IntVar(&replicaAsk, "replicas", 1, "number of replicas, you may want to deploy.")
	flag.BoolVar(&version, "version", false, "display version and exit.")
	flag.BoolVar(&legends, "legends", false, "print legends and exit.")
	flag.Parse()

	// intialize tabwriter for formatted printing
	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 0, 6, 3, ' ', tabwriter.AlignRight)

	if version {
		printversion()
	}

	if legends {
		printLegends(w)
	}

	loadConfig, err := cmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Println("There is a problem loading kubeconfig file!!")
		panic(err.Error())
	}

	newClientSet, err := k8s.NewForConfig(loadConfig)
	if err != nil {
		fmt.Println("There is a problem creating a client!!")
		panic(err.Error())
	}

	/* get nodes details
	   This function does the most of heavy lifting for the program
	   It is a manager function.
	*/
	getNodeResources(w, newClientSet, cpuAsk, memoryAsk, cpuLimitAsk, memoryLimitAsk, replicaAsk)
}

// getNodeResources fetches allocated resources for each nodes.
func getNodeResources(w *tabwriter.Writer, c *k8s.Clientset, cpuAsk string, memoryAsk string, cpuLimitAsk string, memoryLimitAsk string, replicaAsk int) {

	var header bool

	namespaceList := make([]string, 0, 3)
	// get list of namespaces
	namespaces, err := c.CoreV1().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		fmt.Println("There is a problem getting namespaces!!")
		panic(err.Error())
	}

	// prepare the list of namspaces
	for n := 0; n < len(namespaces.Items); n++ {
		namespaceList = append(namespaceList, namespaces.Items[n].Name)
	}

	// get node list based on node status, should not be unknown
	nodes, err := c.CoreV1().Nodes().List(metav1.ListOptions{})
	if err != nil {
		fmt.Println("There is a problem getting node list!!")
		panic(err.Error())
	}

	// initialize lists and maps
	unHealthyNodes, temp := make([]string, 0, 3), "ok"
	netReplicas, master, node, overcommittedNodes := int64(0), int64(0), int64(0), make([]string, 0, 3)
	conditions := make(map[string]string)

	// check for healthy nodes
	for n := 0; n < len(nodes.Items); n++ {
		for c := 0; c < len(nodes.Items[n].Status.Conditions); c++ {
			conditionType := string(nodes.Items[n].Status.Conditions[c].Type)
			conditions[conditionType] = string(nodes.Items[n].Status.Conditions[c].Status)
		}

		/* A node is healthy if it is not under any pressure and posting a Ready status.
		   Some of the status from Node Life cyle are;
		   1. Disk pressure
		   2. Memory Pressure
		   3. Cpu pressure
		*/
		for key, value := range conditions {
			if value == "False" && key != "Ready" {
				temp = "ok"
			} else if value == "True" && key == "Ready" {
				temp = "ok"
			} else {
				temp = "bad"
			}
		}

		if temp == "ok" {
			if nodes.Items[n].Labels["node-role.kubernetes.io/node"] == "true" {
				node++
				// get accumulated allocation of cpu and memory
				cpuReq, cpuLimit, memoryReq, memoryLimit, totalPods := calculatePodResources(c, nodes.Items[n].Name, namespaceList)

				cap := nodes.Items[n].Status.Capacity
				alloc := nodes.Items[n].Status.Allocatable

				// get capacity and allocation
				podCapacity := cap.Pods().Value()

				podAllocatable := alloc.Pods().Value()
				nodeCPUCapacity := cap.Cpu().MilliValue()
				nodeMemoryCapacity := cap.Memory().Value()

				nodeCPUAllocatable := alloc.Cpu().MilliValue()
				nodeMemoryAllocatable := alloc.Memory().Value()

				remainingCPUReq := nodeCPUAllocatable - cpuReq
				TotalCPUAsk := cpuToInt64(cpuLimitAsk) + cpuLimit

				cpuLimitAskPercentage := TotalCPUAsk / nodeCPUCapacity * 100

				remainingMemoryReq := nodeMemoryAllocatable - memoryReq
				TotalMemoryAsk := ToBytes(memoryLimitAsk) + memoryLimit

				memoryLimitAskPercentage := TotalMemoryAsk / nodeMemoryCapacity * 100

				// print header for the first time and ensure, it doesn't repeat.
				if !header {
					if printHeader(w) {
						header = true
					}
				}

				/* Once we get the number of spinnable pods per node, we should
				determine if the requested number of replicas be achiveved in
				the cluster, assuming there is no port constraint.
				*/
				spinnable, overcommittedNode := calculateCapacity(strings.SplitAfterN(nodes.Items[n].Name, ".", 2)[0],
					nodeCPUCapacity,
					nodeMemoryCapacity,
					podCapacity,
					nodeCPUAllocatable,
					nodeMemoryAllocatable,
					podAllocatable,
					cpuReq,
					cpuLimit,
					memoryReq,
					memoryLimit,
					totalPods,
					remainingCPUReq,
					remainingMemoryReq,
					cpuToInt64(cpuAsk),
					ToBytes(memoryAsk),
					replicaAsk,
					memoryLimitAskPercentage,
					cpuLimitAskPercentage,
					w)

				netReplicas = netReplicas + spinnable

				if overcommittedNode != "nil" {
					overcommittedNodes = append(overcommittedNodes, overcommittedNode)
				}
			} else if nodes.Items[n].Labels["node-role.kubernetes.io/master"] == "true" || nodes.Items[n].Labels["node-role.kubernetes.io/master"] == "" {
				master++
			}
		} else {
			unHealthyNodes = append(unHealthyNodes, strings.SplitAfterN(nodes.Items[n].Name, ".", 2)[0])
		}
	}

	Columns(w, "\n")
	// if  there are no worker nodes, print a warning message. TODO: format this message properly.
	if node == 0 {
		fmt.Println(" W: Number of worker nodes are 0!!!")
		fmt.Printf(" %s\n\n", "W: To use this program either add a worker node or label one of the masters with 'node-role.kubernetes.io/node=true'!!!")
	}

	Rows(w, "%s\t%d\t%s\t%d\n", "Number of Master Nodes: ", master, "Number of worker nodes: ", node)
	Rows(w, "%s\t%s\t%s\t%s\n", "Memory Request via STDIN: ", memoryAsk, "Memory Limit via STDIN: ", memoryLimitAsk)
	Rows(w, "%s\t%s\t%s\t%s\n", "CPU Request via STDIN: ", cpuAsk, "CPU Limit via STDIN: ", cpuLimitAsk)

	if netReplicas >= int64(replicaAsk) {
		Rows(w, "%s\t%d\t%s\t%s\n", "Replica Requested via STDIN: ", replicaAsk, "Is Schedulable?: ", "True")
	} else {
		Rows(w, "%s\t%d\t%s\t%s\n", "Replica Requested via STDIN: ", replicaAsk, "Is Schedulable?: ", "False")
	}

	Columns(w, "\n")
	Rows(w, "%s\t%d\t\n", "Nodes With OverCommitted CPU/Memory: ", len(overcommittedNodes))
	Rows(w, "%s\t%s\t", "Overcommitted Nodes List: ", VPrint(overcommittedNodes))
	Columns(w, "\n")

	Rows(w, "%s\t%d\t\n", "Unhealthy Nodes: ", len(unHealthyNodes))

	Rows(w, "%s\t%s\t", "Unhealthy Nodes List: ", VPrint(unHealthyNodes))
	Columns(w, "\n")
	w.Flush()
}

// calculatePodResources calculates resources currentky consumed by each pod.
func calculatePodResources(c *k8s.Clientset, nodeName string, nsList []string) (int64, int64, int64, int64, int) {

	var podLength int
	podLength = 0

	// set condition to identify the non terminted pods, based on the Pods Life Cycle
	fieldSelector, err := fields.ParseSelector("spec.nodeName=" + nodeName + ",status.phase!=" + "Pending" + ",status.phase!=" + "Succeeded" + ",status.phase!=" + "Failed" + ",status.phase!=" + "Unknown")
	if err != nil {
		fmt.Println("There is a problem setting filters!!")
		panic(err.Error())
	}
	// initialize the variables
	request, reqlimit, cpureq, memoryreq, cpulimit, memorylimit := int64(0), int64(0), int64(0), int64(0), int64(0), int64(0)

	// check all namespaces on individual nodes and loop through containers to get allocations at container level.
	for s := 0; s < len(nsList); s++ {
		pods, err := c.CoreV1().Pods(nsList[s]).List(metav1.ListOptions{FieldSelector: fieldSelector.String()})
		if err != nil {
			fmt.Println("There is a problem getting pods!!")
			panic(err.Error())
		}
		for _, p := range pods.Items {
			for _, container := range p.Spec.Containers {
				// get limits and requests and sum them up
				request = container.Resources.Requests.Cpu().MilliValue()
				memory := container.Resources.Requests.Memory().Value()
				reqlimit = container.Resources.Limits.Cpu().MilliValue()
				memlimit := container.Resources.Limits.Memory().Value()

				cpureq += request
				memoryreq += memory
				cpulimit += reqlimit
				memorylimit += memlimit
			}
			podLength++
		}
	}

	return cpureq, cpulimit, memoryreq, memorylimit, podLength
}

// cpuToInt64 converts string data to integer to represents CPU units in milicores
func cpuToInt64(data string) int64 {
	/*
	  This function is taken from bytes.go and modified to behave
	  as needed for our requoirement, here we handle the input given
	  in milicores or cores.
	*/

	n := strings.IndexFunc(data, unicode.IsLetter)

	switch n {
	case -1:
		cores, err := strconv.ParseInt(data, 10, 64)
		if err != nil {
			fmt.Println("There is a problem in parsing input for cores!!")
			panic(err.Error())
		}
		milicores := cores * 1000
		return milicores
	default:
		milicores, _ := strconv.ParseInt(data[:n], 10, 64)
		return milicores
	}
}

// ToMegabytes converts string input to megabytes
func ToMegabytes(s string) int64 {
	bytes := ToBytes(s)
	return bytes / MEGABYTE
}

// ToBytes converts string values to bytes
func ToBytes(s string) int64 {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)

	i := strings.IndexFunc(s, unicode.IsLetter)

	if i == -1 {
		return 0
	}

	bytesString, multiple := s[:i], s[i:]
	bytes, err := strconv.ParseFloat(bytesString, 64)
	if err != nil || bytes <= 0 {
		return 0
	}

	switch multiple {
	case "T", "TB", "TIB", "TI":
		return int64(bytes * TERABYTE)
	case "G", "GB", "GIB", "GI":
		return int64(bytes * GIGABYTE)
	case "M", "MB", "MIB", "MI":
		return int64(bytes * MEGABYTE)
	case "K", "KB", "KIB", "KI":
		return int64(bytes * KILOBYTE)
	case "B":
		return int64(bytes)
	default:
		return 0
	}
}

// calculateCapacity calculates current usage and maximum number of spinnable pods per node and return
func calculateCapacity(node string,
	nodeCPUCapacity int64,
	nodeMemoryCapacity int64,
	podCapacity int64,
	nodeCPUAllocatable int64,
	nodeMemoryAllocatable int64,
	podAllocatable int64,
	cpuReq int64,
	cpuLimit int64,
	memoryReq int64,
	memoryLimit int64,
	totalPods int,
	remainingCPUReq int64,
	remainingMemoryReq int64,
	cpuAsk int64,
	memoryAsk int64,
	replicaAsk int,
	memoryLimitAskPercentage int64,
	cpuLimitAskPercentage int64,
	p *tabwriter.Writer) (int64, string) {

	fractionNODECPUReq := float64(cpuReq) / float64(nodeCPUAllocatable) * 100
	fractionNodeMemoryReq := float64(memoryReq) / float64(nodeMemoryAllocatable) * 100
	fractionNODECPULimit := float64(cpuLimit) / float64(nodeCPUAllocatable) * 100
	fractionNodeMemoryLimit := float64(memoryLimit) / float64(nodeMemoryAllocatable) * 100

	spinnable, cpuCrunch, memoryCrunch := IsSpinnable(remainingCPUReq, remainingMemoryReq, cpuAsk, memoryAsk, replicaAsk, podAllocatable)

	if spinnable == podAllocatable {
		spinnable = spinnable - int64(totalPods)
	}

	Rows(p, "%2s\t%4.2f%%\t%4.2f%%\t%6.2f%%\t%7.2f%%\t%5d\t%11t\t%10t\t%6d\t\n", node, fractionNODECPUReq, fractionNodeMemoryReq, fractionNODECPULimit, fractionNodeMemoryLimit, totalPods, cpuCrunch, memoryCrunch, spinnable)

	if memoryLimitAskPercentage > 110 || cpuLimitAskPercentage > 100 || int64(fractionNODECPULimit) > 110 || int64(fractionNodeMemoryLimit) > 100 {
		return spinnable, node
	}

	return spinnable, "nil"
}

// IsSpinnable Test how many more pods can be spun with same resources given, per node.
// it calculates the remaining resources out of the fetched information.
// it also helps in calculating the number of maximum pods that can be spun with the remaining resources.
func IsSpinnable(remainingCPUReq int64, remainingMemoryReq int64, cpuAsk int64, memoryAsk int64, replicaAsk int, podAllocatable int64) (int64, bool, bool) {

	/*
	  We should be good if:
	  1. The remianing memory AND remaining CPU is greater than the current usage + what was requested.
	  2. The node can take minimum 1 replica with the specification provided.
	*/
	if remainingCPUReq >= cpuAsk && remainingMemoryReq >= memoryAsk {

		if (remainingMemoryReq / memoryAsk) > (remainingCPUReq / cpuAsk) {
			if podAllocatable > (remainingCPUReq/cpuAsk) && (remainingCPUReq/cpuAsk) >= int64(1) {
				return remainingCPUReq / cpuAsk, false, false
			} else if podAllocatable < (remainingCPUReq / cpuAsk) {
				return podAllocatable, false, false
			}
		} else if (remainingMemoryReq / memoryAsk) < (remainingCPUReq / cpuAsk) {
			if podAllocatable > (remainingMemoryReq/memoryAsk) && (remainingMemoryReq/memoryAsk) >= int64(1) {
				return remainingMemoryReq / memoryAsk, false, false
			} else if podAllocatable < (remainingMemoryReq / memoryAsk) {
				return podAllocatable, false, false
			}
		}
	} else if remainingCPUReq < cpuAsk {
		return 0, true, false
	} else if remainingMemoryReq < memoryAsk {
		return 0, false, true
	}

	return 0, true, true
}

// getKubeConfig sets the path of kubeconfg file.
func getKubeConfig() string {
	// try read config file from environment.
	if config := os.Getenv("KUBECONFIG"); config != "" {
		return config
	} else if homedir := os.Getenv("HOME"); homedir != "" {
		return filepath.Join(homedir, ".kube", "config")
	} else {
		return "none"
	}
}

// Columns create columns for the output table.
func Columns(p *tabwriter.Writer, out string) {

	fmt.Fprintln(p, out)
}

// Rows creates and format rows for the output table.
func Rows(p *tabwriter.Writer, format string, out ...interface{}) {

	fmt.Fprintf(p, format, out...)
}

// printHeader prints the formatted header for the output from this program.
func printHeader(p *tabwriter.Writer) bool {
	Columns(p, "\n")
	Rows(p, "%-2s\n", "  +-------------------------------------------------------------------------------------------------+")
	Rows(p, "%40s\t%50s\n", "Current Capacity Usage Per Node", "Spinnable Pods")
	Rows(p, "%-2s\n", "  +-------------------------------------------------------------------------------------------------+")
	Rows(p, "%-11s\t%-5s\t%-5s\t%-5s\t%-5s\t%-5s |\t%-5s\t%-5s\t%-5s\t\n", "Node", "CpuReq", "MemReq", "CpuLimit", "MemLimit", "Pods", "CpuCrunch", "MemCrunch", "Spinnable")
	Rows(p, "%-2s\n", "  +-------------------------------------------------------------------------------------------------+")
	p.Flush()

	return true
}

// VPrint vertically print the input list
type VPrint []string

func (s VPrint) String() string {
	var str string
	for _, i := range s {
		str += fmt.Sprintf("\t\t%s\n", i)
	}
	return str
}

// printversion prints the version and build date information.
func printversion() {
	fmt.Printf("%s\t%s\n%s\t%s\n", "version:", version, "buildDate:", buildDate)
	os.Exit(0)
}

// printLegends prints the legends against the output fields of this program.
func printLegends(p *tabwriter.Writer) {
	Columns(p, "\n")
	Rows(p, "%s\t\n", "+++++++ Legends +++++++")
	Columns(p, "\n")
	Rows(p, "%s\t%s\n", "Is Schedulable?: ", "if 'true', Pods can be spun on worker node with the amount of CPU and Memory requested. False, otherwise.")
	Rows(p, "%s\t%s\n", "Overcommitted Nodes List: ", "List of nodes which will OverCommit CPu/Memory Limits with the amount of CPU and Memory requested.")
	Rows(p, "%s\t%s\n", "Unhealthy Nodes List: ", "List of nodes which are not healthy or which reached either disk/memory/cpu load.")
	Columns(p, "\n")
	Rows(p, "%s\t\n", "Understanding Spinnable Pods")
	Rows(p, "%-2s\n", "  +----------------------------------------------+")
	Rows(p, "%s\t%s\n", "CpuCrunch: ", "if 'true', amount of CPU requested is not available on the worker node.")
	Rows(p, "%s\t%s\n", "MemCrunch: ", "if 'true', amount of Memory requested is not available on the worker node.")
	Rows(p, "%s\t%s\n", "Spinnable: ", "maximum number of pods that can be spun on worker node with the amount of CPU and Memory requested.")
	Columns(p, "\n")
	p.Flush()
	Rows(p, "%s\t\n", "Understanding Current Capacity Usage Per Node")
	Rows(p, "%-2s\n", "  +----------------------------------------------+")
	Rows(p, "%s\t%s\n", "Pods: ", "total number of pods currently running on worker node.")
	Rows(p, "%s\t%s\n", "Nodes: ", "kubernetes cluster worker node names.")
	Rows(p, "%s\t%s\n", "CpuReq: ", "amount of CPU allocated on worker node, at present.")
	Rows(p, "%s\t%s\n", "MemReq: ", "amount of Memory allocated on worker node, at present.")
	Rows(p, "%s\t%s\n", "CpuLimit: ", "amount of CPU Limit set on worker node, at present.")
	Rows(p, "%s\t%s\n", "MemLimit: ", "amount of Memory Limit set on worker node, at present.")
	p.Flush()

	os.Exit(0)
}
