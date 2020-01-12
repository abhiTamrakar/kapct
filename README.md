# kapct
A project determining the cluster capacity the way kubernetes sees it.

This project calculates maximum number of pods that can be scheduled by caulating the remaining resources on each worker node* in the cluster.

It does not calculates resources used by master as long as master is not amongst labeled nodes.

## USAGE
$ kapct [options]

options:

-cpulimit string
    amount of CPU you desire in m(milicores), use only string formatted interger for cores. (default "100m")
-cpureq string
    amount of CPU you desire in m(milicores), use only string formatted interger for cores. (default "100m")
-kubeconfig string
    (optional) absolute path to the kubeconfig file (default "/home/ankita/.kube/config")
-legends
    print legends and exit.
-memlimit string
    amount of memory you desire in K(KB),M(MB),G(GB),T(TB) (default "1G")
-memreq string
    amount of memory you desire in K(KB),M(MB),G(GB),T(TB) (default "1G")
-replicas int
    number of replicas, you may want to deploy. (default 1)
-version
    display version and exit.
