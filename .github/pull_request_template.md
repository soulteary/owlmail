## Pull Request Description / 变更描述

<!-- Clearly describe what and why. / 清晰说明改了什么以及原因。 -->

## Type of Change / 变更类型

<!-- Please check the applicable options -->

- [ ] 🐛 Bug fix / Bug 修复
- [ ] ✨ New feature / 新功能
- [ ] 💥 Breaking change / 破坏性变更
- [ ] 📝 Documentation / 文档
- [ ] 🎨 Code style / 代码风格
- [ ] ♻️ Refactoring / 重构
- [ ] ⚡ Performance / 性能
- [ ] ✅ Tests / 测试
- [ ] 🔧 Build or tooling / 构建或工具
- [ ] 🔄 Other / 其他

## Related Issue / 相关 Issue

<!-- If this PR addresses an issue, please reference it here -->
<!-- Use format "Fixes #123" or "Closes #123" -->

Fixes #

## Change Details / 变更详情

<!-- Describe your changes in detail -->

### New Features / 新功能
- 

### Bug Fixes / Bug 修复
- 

### Breaking Changes / 破坏性变更
- 

## Testing / 测试

<!-- Describe the tests you ran to verify your changes -->

- [ ] I have run all existing tests
- [ ] I have added new tests to cover my changes
- [ ] All tests pass

### Test Commands
```bash
go test -race ./...
go vet ./...
bun build ./web/*.js --target=browser --outdir=./.bun-check
bun test ./tests/web ./tests/docs
```

### Test Results / 测试结果
<!-- Paste test output or screenshots. / 粘贴测试输出或截图。 -->

## Compatibility and Migration / 兼容性与迁移

<!-- Describe native API, MailDev/MailCatcher facade, configuration, stored data,
and upgrade impact, or write "None". / 说明原生 API、兼容层、配置、存量数据与升级影响，
没有影响请写 None。 -->

## Checklist / 检查清单

<!-- Please ensure all items are completed -->

- [ ] Code style and self-review completed / 已完成代码风格检查与自审
- [ ] Relevant tests and documentation updated / 已更新相关测试与文档
- [ ] New and existing tests pass without new warnings / 新旧测试通过且没有新增警告
- [ ] Dependent changes are identified and available / 已说明并提交依赖变更
- [ ] No passwords, SMTP/S3 credentials, Webhook secrets, tokens, or sensitive email content / 不含密码、SMTP/S3 凭据、Webhook Secret、Token 或敏感邮件内容

## Screenshots / 截图（如适用）

<!-- If your changes involve UI, please add screenshots -->

## Additional Information / 附加信息

<!-- Any other relevant information such as dependencies, migration steps, etc. -->
