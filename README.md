# kubectl-kruise
kubectl plugin for OpenKruise

`kubectl` supports a plug-in mechanism, but the rollout and other related operations provided by this tool itself only support the native workload resources of Kubernetes.
Therefore, we need to create a kubectl plugin for OpenKruise, through which community users can use kubectl to operate Kruise’s workload resources.

So, `kubectl-kruise` was created.

### How to use
The development of  `kubectl-kruise`  is in progress, if you wanna to experience it, you can clone it and make it:

```
make build && cp kubectl-kruise /usr/local/bin

```

Then you can operate Openkruise resource by `kubectl-kruise`.
By now the `rollout undo`, `rollout status`, `rollout history` has been developed.

![](https://tva1.sinaimg.cn/large/008i3skNgy1gqmmcx5nlqj31eo0je420.jpg)

### TODO
#### kubectl kruise rollout for CloneSet workload
   * [x] undo
   * [x] history
   * [x] status
   * [x] pause
   * [x] resume
   * [x] restart
   
#### kubectl kruise rollout for Advanced StatefulSet
   * [x]  undo
   * [x] history
   * [x] status
   * [x] pause
   * [x] resume
   * [x] restart
   
#### kubectl kruise set SUBCOMMAND [options] for CloneSet
   * [x] kubectl kruise set image cloneset/abc
   * [x] kubectl kruise set env cloneset/abc
   * [x] kubectl kruise set serviceaccount cloneset/abc 
   * [x] kubectl kruise set resources cloneset/abc 
   
#### kubectl kruise set SUBCOMMAND [options] for Advanced StatefulSet
   * [x] kubectl kruise set image asts/abc
   * [x] kubectl kruise set env asts/abc
   * [x] kubectl kruise set serviceaccount asts/abc 
   * [x] kubectl kruise set resources asts/abc 
   
#### kubectl kruise top cloneset [cloneset name] 
   * [ ] kubectl kruise top cloneset abc
   
#### kubectl kruise autoscale SUBCOMMAND [options]
   * [x] kubectl kruise autoscale 
 
### Contributing
We encourage you to help out by reporting issues, improving documentation, fixing bugs, or adding new features. 
