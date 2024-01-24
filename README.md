# disko
Disk-o is a Disk Operator for kubernetes. 


# APIs
LVMCluster represents a cluster that supports LVM.
```sh
kubebuilder create api --group storage --version v1 --kind LVMCluster
```

LVMNode represents a kubernetes Node that supports LVM. LVMNode objects are created automatically. 
```sh
kubebuilder create api --group storage --version v1 --kind LVMNode
```
