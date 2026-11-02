# 多人容器部署预览与验证

本目录提供部署布局，**目前生产 CLI 保留多人发布门禁，不能据此启动正式多人服务**。Compose 中不提供环境变量、build arg 或特殊入口来绕过门禁。Stargate 尚未提供可信“邮箱已验证”声明，不能将 `X-Auth-Email` 无条件当作验证证明。

## 布局

- [Compose](compose.yaml) 使用本地源码构建开发镜像，默认只在宿主 loopback 发布端口。
- 一次性初始化容器仅为本例新建的 state 卷设置 UID/GID 1000，应用仍以非 root 运行。
- 邮件在 `/data/mail`，权威控制库在独立目录 `/data/control/control.sqlite`；备份必须包含两者及事务日志。
- 配置、系统账号和证书通过只读挂载提供，不放入镜像。只读挂载仍必须具备 UID 1000 的读取权限。
- 暂不包含 Stargate 服务本身；接入时必须限制后端 HTTP 仅可信代理可达，并剥除客户端身份头。

准备文件时将 [配置模板](owlmail.yaml.example) 和[账号模板](accounts.yaml.example) 分别复制到 `config/owlmail.yaml`、`config/accounts.yaml`，替换域名、代理 CIDR、bcrypt 散列和证书路径。Linux 上由运维将目录及私钥/账号文件授予容器 UID/GID 1000，目录使用 0700，敏感文件使用 0600。不要通过把私钥改为全局可读来修复 UID 映射；macOS/Podman 的映射应按部署环境单独验证。

`docker compose -f examples/multiuser/compose.yaml config --quiet` 只校验布局，不代表运行成功。生产门禁解除前，启动失败是预期结果；本例 `restart: no` 防止反复重启。

## 实际容器回归

在仓库根目录构建生产镜像。`TARGETARCH` 应匹配运行平台，例如 `amd64` 或 `arm64`：

```sh
docker build --build-arg TARGETARCH=amd64 -t owlmail:container-check .
docker build --build-arg BASE_IMAGE=owlmail:container-check --build-arg TARGETARCH=amd64 \
  -f tests/container/Dockerfile.multiuser-fixture -t owlmail:container-fixture .
python3 tests/container/multiuser.py --image owlmail:container-check \
  --fixture-image owlmail:container-fixture
```

生产镜像验证单机 SMTP/附件、HTTP、非 root 权限、重启持久化、打包 healthcheck、离线双向迁移及多人/stdio MCP 门禁。专用 fixture 镜像显式使用测试 build tag，仅在容器内部 loopback 提供合成身份代理，验证真实 TLS SMTP、待认领、隔离和重启。fixture 不得发布或作为生产镜像部署。

脚本仅清理本次唯一命名的容器和带本次标记的卷。Podman 可用 `--runtime podman --connection <连接名>`；测试结果必须明确实际 runtime，不能把 Podman 结果表述为 Docker 已验证。

Gorge 继续使用现有 SMTP 适配器。正式接入时配置系统账号和受信 SMTP 证书，不需要逐用户 SMTP 密码，也不修改 Gorge 代码。
