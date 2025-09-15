<p align="center">
  <img src="./assets/logo.svg" alt="XGit Logo" width="120" height="120" />
</p>

# XGit

✧ 中文描述  
XGit 是一个轻量级 GitHub Web 客户端，专注于 **仓库浏览与代码编辑**。  
它让你在手机或浏览器中，快速浏览仓库、修改文件、提交更改，支持中英文切换，简单高效。  

✧ English Description  
XGit is a lightweight GitHub web client focused on **repository browsing and code editing**.  
It enables you to browse repositories, edit files, and commit changes directly from your phone or browser, with bilingual (EN/中文) support.  

---

## ⚠️ Important Changes (2025-09)

- 所有 **file.* 写类指令**（`file.write/append/prepend/replace/delete/move/chmod` 等）已弃用。  
- 请统一使用 **git.diff** 协议来进行变更，当前支持：
  - A-only（新增）
  - M-only（修改）
  - D-only（删除）
  - R-only（改名，禁止 R+M 组合）
  - Mode-only（权限）

---

## ✨ 特点 Features
- 📂 仓库树浏览
- 📝 在线代码编辑（Monaco Editor 即将接入）
- ✅ 多文件修改、批量提交
- 🌍 多语言支持（中文 / English）
- 🚀 简洁快速，手机端友好

---

## 📊 对比 Comparison
| 功能 / Feature       | GitHub App | XGit |
|----------------------|------------|------|
| 仓库浏览 / Repo Tree | ✅ 基础     | ✅ 更快、轻量 |
| 文件编辑 / Editing   | ❌ 无       | ✅ 支持 |
| 多文件修改 / Multi   | ❌ 无       | ✅ 有 |
| 分支切换 / Branch    | ✅ 支持     | 🚧 开发中 |
| 提交日志 / History   | ✅ 支持     | 🚧 开发中 |
| 多语言 / i18n        | ❌ 无       | ✅ 中文/English |

---

## 📌 版本 Version
当前版本：v0.1.0  
查看 [更新日志](./docs/changelog.md)

---

## 📜 License
MIT License © 2025 Ximory
