# 贡献指南

感谢您对 OwlMail 项目的关注！我们欢迎所有形式的贡献。

## 如何贡献

### 报告问题

如果您发现了 bug 或有功能建议，请：

1. 检查 [Issues](https://github.com/soulteary/owlmail/issues) 中是否已有相关问题
2. 如果没有，请创建新的 Issue，使用相应的模板
3. 提供尽可能详细的信息，包括：
   - 问题描述
   - 复现步骤
   - 预期行为
   - 实际行为
   - 环境信息（操作系统、Go 版本等）

### 提交代码

1. **Fork 仓库**
   ```bash
   git clone https://github.com/soulteary/owlmail.git
   cd owlmail
   ```

2. **创建分支**
   ```bash
   git checkout -b feature/your-feature-name
   # 或
   git checkout -b fix/your-bug-fix
   ```

3. **进行更改**
   - 编写清晰的代码
   - 遵循项目的代码风格
   - 添加必要的测试
   - 更新相关文档

4. **运行测试**
   ```bash
   # 运行所有测试
   go test ./...
   
   # 运行测试并查看覆盖率
   go test -cover ./...
   
   # 运行特定包的测试
   go test ./internal/api/...
   ```

5. **提交更改**
   ```bash
   git add .
   git commit -m "feat: 添加新功能描述"
   # 或
   git commit -m "fix: 修复问题描述"
   ```

   提交信息应遵循 [Conventional Commits](https://www.conventionalcommits.org/) 规范：
   - `feat:` 新功能
   - `fix:` Bug 修复
   - `docs:` 文档更改
   - `style:` 代码格式（不影响代码运行的变动）
   - `refactor:` 重构（既不是新增功能，也不是修复 bug）
   - `test:` 添加或修改测试
   - `chore:` 构建过程或辅助工具的变动

6. **推送并创建 Pull Request**
   ```bash
   git push origin feature/your-feature-name
   ```
   
   然后在 GitHub 上创建 Pull Request，填写 PR 模板中的信息。

## 代码规范

### Go 代码风格

- 遵循 [Effective Go](https://go.dev/doc/effective_go) 和 [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- 使用 `gofmt` 格式化代码
- 使用 `golint` 检查代码风格
- 保持函数简洁，单一职责

### 测试要求

- 新功能必须包含测试
- Bug 修复应包含回归测试
- 测试覆盖率不应降低
- 使用表驱动测试（Table-Driven Tests）处理多个测试用例

### 文档要求

- 公共 API 必须有文档注释
- 复杂逻辑应添加注释说明
- 更新相关的 README 或文档

## 开发环境设置

### 前置要求

- Go 1.24 或更高版本
- Git

### 设置步骤

1. Fork 并克隆仓库
   ```bash
   git clone https://github.com/YOUR_USERNAME/owlmail.git
   cd owlmail
   ```

2. 添加上游仓库
   ```bash
   git remote add upstream https://github.com/soulteary/owlmail.git
   ```

3. 安装依赖
   ```bash
   go mod download
   ```

4. 运行测试确保一切正常
   ```bash
   go test ./...
   ```

## Pull Request 流程

1. 确保您的分支基于最新的 `main` 分支
   ```bash
   git checkout main
   git pull upstream main
   git checkout your-branch
   git rebase main
   ```

2. 确保所有测试通过
   ```bash
   go test ./...
   ```

3. 确保代码格式化
   ```bash
   gofmt -w .
   ```

4. 创建 Pull Request
   - 使用清晰的标题和描述
   - 链接相关的 Issue（如果存在）
   - 描述您的更改和原因
   - 添加测试截图或示例（如果适用）

5. 等待代码审查
   - 维护者会审查您的 PR
   - 可能需要一些修改
   - 请及时响应审查意见

## 项目结构

```
OwlMail/
├── cmd/
│   └── owlmail/          # 主程序入口
├── internal/
│   ├── api/              # Web API 实现
│   ├── common/           # 通用工具（日志、错误处理）
│   ├── maildev/          # MailDev 兼容层
│   ├── mailserver/       # SMTP 服务器实现
│   ├── outgoing/         # 邮件转发实现
│   └── types/            # 类型定义
├── web/                  # Web 前端文件
└── .github/              # GitHub 配置文件
```

## 贡献类型

我们欢迎以下类型的贡献：

- 🐛 **Bug 修复**：修复现有功能的问题
- ✨ **新功能**：添加新功能或改进现有功能
- 📝 **文档**：改进文档、添加示例或教程
- 🎨 **UI/UX**：改进 Web 界面
- ⚡ **性能**：性能优化
- 🧪 **测试**：添加或改进测试
- 🔧 **工具**：改进开发工具或构建流程

## 问题

如果您在贡献过程中遇到任何问题，请：

1. 查看现有的 [Issues](https://github.com/soulteary/owlmail/issues)
2. 在 [Discussions](https://github.com/soulteary/owlmail/discussions) 中提问
3. 创建新的 Issue 描述您的问题

## 行为准则

请遵循我们的 [行为准则](CODE_OF_CONDUCT.md)，以保持社区友好和尊重。

## 许可证

通过贡献，您同意您的贡献将在与项目相同的 [MIT 许可证](LICENSE) 下授权。

---

再次感谢您对 OwlMail 的贡献！🦉

