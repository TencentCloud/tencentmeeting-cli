# 贡献指南

感谢你对 tmeet 的关注！我们欢迎任何形式的贡献。

## 提交 Issue

- 提交前请先搜索是否已有相关 Issue
- 请清晰描述问题现象、复现步骤和期望行为
- 如涉及安全漏洞，请勿公开提交，参阅 [SECURITY.md](SECURITY.md)

## 提交 Pull Request

1. Fork 本仓库并创建你的分支：`git checkout -b feat/your-feature`
2. 确保代码通过格式检查：`gofmt -l .`
3. 确保所有测试通过：`go test ./...`
4. 提交时写清楚变更说明
5. 发起 Pull Request，描述你的改动

## 代码规范

- 使用 `gofmt` 格式化代码
- 注释使用中文，遵循 Go doc 规范
- 新增功能需附带单元测试
- 敏感信息（密钥、token）禁止硬编码或明文落盘

## 分支策略

- `main`：稳定发布分支
- `feat/*`：新功能开发
- `fix/*`：Bug 修复
