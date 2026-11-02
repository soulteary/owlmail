# 隔离 NFSv4 ACL 实验室

本实验室验证原生 NFSv4 有序 ACE、实际身份访问、继承及服务端备份恢复。它是手动运行的独立测试设施，不是 OwlMail 的生产存储部署方式，也不解除多人发布门禁。单机默认配置和 Gorge 无需变化。

## 选择与边界

使用 LizardFS 原生 RichACL 后端，通过 Ganesha 的 FSAL_LIZARDFS 暴露 NFSv4.1。不能把 POSIX ACL 映射或仅能挂载的服务当作原生 NFSv4 ACL 验证。[Ganesha 官方 ACL 支持矩阵](https://github.com/nfs-ganesha/nfs-ganesha/wiki/ACL-Support)说明了各 FSAL 的差异。

服务端在既有 Linux Podman VM 的 rootless 容器中运行，完整状态保存在本轮独有的数据卷。仅将 NFS 端口发布到 VM 的 `127.0.0.1` 随机高端口。LizardFS 的 master/chunkserver 使用容器内部地址通信，原生导出只允许该地址；不改宿主 exports、身份配置、SELinux 策略或防火墙。

客户端是同一 VM 内独立 rootful 容器，需要 `SYS_ADMIN` 执行真实内核挂载，使用私有 mount namespace，并检查传播属性。为访问 VM loopback，客户端使用 host network，因此不能声称网络 namespace 完全隔离。客户端不是 privileged，不共享宿主 PID namespace。UID 1001/1002 仅存在于测试容器；AUTH_SYS 使用数字身份，必须实际验证新文件 owner 与对应进程的 UID/GID，不能接受匿名身份恰好通过。

这不覆盖 Kerberos、跨域身份映射、独立双宿主、pNFS 或服务端故障后的 NFS 会话 reclaim；测试服务明确使用 Graceless。也不证明 OwlMail SQLite 可以直接放到 NFS 上。独立双宿主验收项继续保留。若显式禁用客户端容器标签，报告必须记录该选项；宿主 Enforcing 不能关闭。

## 测试组件与顺序

- `Dockerfile.nfs4_lab_tools`、`Dockerfile.nfs4_lab_server`：构建固定上游源码和原生后端，不使用 MEM FSAL 或模拟 ACL。
- `nfs4_lab_server.py`：仅初始化带本轮标记的新状态，启动 master/chunkserver/Ganesha，输出 ready 信息，逐项优雅停止；非空未标记状态和缺失恢复元数据必须报错。
- `nfs4_client_check.py`：确认真实 NFSv4 挂载和原生 ACL 接口。这只是能力检查。
- `nfs4_client_acceptance.py`：先拒绝后允许、先允许后拒绝、另一身份拒绝；真实 read/write/fsync、固定预期内容和非 root O_DIRECT 分偏移读取（不支持即失败，无缓存读取回退）、有序文本及原始 ACL、UID/GID；实际创建的子项和孙项继承。
- `nfs4_client_runner.py`：控制独有客户端、私有挂载和退出清理，并保留失败证据。
- `nfs4_lab.py`：先完成 prepare，并以全新客户端挂载 confirm 已知内容和 ACL，优雅停止所有服务，备份整个 native 后端，再恢复到独立新卷；新端口和新客户端挂载执行 verify，并在恢复后新建文件验证继承。最后检查原始源卷及归档未改变，清理本轮资源。

仅比较两份相同的损坏内容不算通过。协议成功、SETACL 成功、能列出 ACE 或相对散列一致，都不能替代真实身份 IO 与已知内容验证。失败必须保留为 failed；未具备环境只能记 unsupported，不能写 passed。

## 构建与运行

需要已有 Podman machine、rootless/rootful 两个连接，以及 VM 的 NFSv4 内核客户端能力；这不是通用 Docker CI 中可无条件执行的任务。构建需要联网访问固定上游源码。macOS Documents 若受文件共享限制，可将本目录构建上下文复制到操作者的专用临时目录，不能将上下文错误归为协议失败。

```sh
podman --connection podman-machine-default build \
  -f tests/container/Dockerfile.nfs4_lab_tools \
  -t localhost/owlmail:nfs4-lab-tools tests/container
podman --connection podman-machine-default build \
  -f tests/container/Dockerfile.nfs4_lab_server \
  -t localhost/owlmail:nfs4-lab-server tests/container
podman --connection podman-machine-default build \
  -f tests/container/Dockerfile.nfs4_client \
  -t localhost/owlmail:nfs4-lab-client tests/container
# 两个连接的镜像库不同，先将客户端镜像导入 rootful 库：
podman --connection podman-machine-default save localhost/owlmail:nfs4-lab-client \
  -o /tmp/owlmail-nfs4-client.tar
podman --connection podman-machine-default-root load -i /tmp/owlmail-nfs4-client.tar
python3 tests/container/nfs4_lab.py \
  --connection podman-machine-default \
  --client-connection podman-machine-default-root \
  --evidence-directory /tmp/owlmail-native-nfs4-new-run
```

使用新的证据目录；不要将失败运行目录覆盖成通过。构建镜像和缓存可保留用于复现，正在运行的容器、挂载、临时数据卷必须按本轮完整资源 ID 清理。不要全局 prune 或删除其他服务资源。

## 上游兼容修补

实验室源码修补仅进入专用测试镜像，必须记录固定提交和每份 patch 的 SHA256。包括 AArch64 指针存储、客户端错误码重载、共享库依赖、Ganesha FSAL ABI/句柄初始化、ACL 返回字段及实际 IO 长度/EOF 报告等兼容问题。不能把改动后端权限算法、硬编码允许、绕过身份检查或关闭实际 fsync 用作通过手段。实际运行结果及仍未覆盖的项目见[实施进度](Multiuser-Progress.md)。

现场曾在 `mkdir` 后读取属性得到 `EREMOTEIO`；定向日志证明 GETATTR 本身成功，而相邻 `GET_DIR_DELEGATION` 返回 OP_ILLEGAL。该症状对应 [Ganesha 官方问题 #1385](https://github.com/nfs-ganesha/nfs-ganesha/issues/1385)，实验室回移 [98eb77c 的两项 dispatcher 修正](https://github.com/nfs-ganesha/nfs-ganesha/commit/98eb77c4fd94effa6882b214698a77752ec79f6e)，让已知但未实现的可选操作返回 NOTSUPP。没有通过修改宿主 nfsv4 模块参数关闭目录委托，也没有将“无 ACL”泛化成允许。最终镜像已通过下述完整验收。

第一次完整准备运行在目录枚举处严格失败。随后专用实例与匹配本轮进程的崩溃栈证明，原生客户端的目录会话未注册，READDIR 命中了现有断言并终止服务。修补依据同源 FUSE 的注册/更新/释放生命周期，保留断言和权限检查；不使用手写路径清单替代真实目录枚举。失败报告和清理证据单独保留，修补后必须新建环境重新执行全流程。

后续独立恢复运行又暴露空枚举：原生偏移 0 返回全部真实条目，1/2 返回空；真实后续 cookie 是 opaque 64 位值，不是条目序号。FSAL 因初始偏移 2 和加减 cookie 的旧逻辑漏掉目录内容。验收因此增加实际遍历的 9 对象精确集合检查，并以独立的大目录覆盖跨原生 100 条批次；恢复快照不能靠固定路径读取或空集合相等通过。桥接必须从原生 0 开始、原样延续 cookie，过滤虚拟点目录，不改变 ACL 或后端数据。

## 本机实际结果与保留项

最终运行 `c72c26a6c6d8` passed，镜像 `95bf3b1ead87ffd569154e277a2790178f46151fd2ef3d0df8dfcc1773a7f40d`。prepare、全新挂载的只读 confirm、独立新卷和新端口的 verify 全部通过。真实快照含全部 9 个对象；另有 137 个长文件名文件跨原生 100 条批次枚举，名称无缺漏/重复，固定内容逐项一致。UID 1001 的真实创建归属、非 root 零 capabilities、有序允许/拒绝、UID 1002 拒绝、原始 ACL 与文本顺序、子孙继承、O_DIRECT 多偏移读取和 fsync 均有证据。恢复后新子项的原始 ACL 散列与源子项精确相同。

完整后端归档包含 master 元数据/日志、chunk 数据、配置与恢复状态；恢复启动前目录 manifest 精确相同。源卷和归档在演练后未变。两套服务各自三进程正常退出，无强杀；客户端实际卸载成功，本轮容器/数据卷残留为 0。构建镜像与缓存保留用于复现。此前诊断实例逐 run 精确清理，不删除其他环境资源。

第三次完整运行曾在恢复端初始化失败，只有监听端口而未建立导出。该次失败证据保留；新增异常诊断、单次有界 native 初始化/根 GETATTR 探针与“本次启动”导出 marker 检查，明确拒绝旧备份日志和空导出假就绪。第四次严格流程通过，但第三次具体异常没有复现，不能把就绪检查加强误称为已确定其底层根因。

原生测试栈仍有上游限制：停止阶段保留了 Ganesha FSAL 的负 fd 计数警告，没有降低日志级别隐藏；本次固定内容、完整快照、真实 fsync 与正常停止均有独立证据，但不据此认定该旧 FSAL 适合生产或任意规模。独立双宿主、Kerberos/跨域映射、会话 reclaim、生产介质和策略可移植性继续保留。默认单机兼容，多人门禁关闭，Gorge 未修改。
