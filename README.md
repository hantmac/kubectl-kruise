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

Then you can operate Openkruise resource by `kubectl-kruise`
By now the `rollout undo`, `rollout history` has been developed.

![](https://tva1.sinaimg.cn/large/008i3skNgy1gqlonz4irxj31h20ro41z.jpg)

### Development progress
#### rollout
   * [x]  undo
   * [x] history
   * [ ] status
   * [ ] pause
   * [ ] resume
   * [ ] restart
   
### Contributing
We encourage you to help out by reporting issues, improving documentation, fixing bugs, or adding new features. 