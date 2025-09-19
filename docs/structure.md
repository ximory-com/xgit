# 目录结构
```
xgit/
 ├─ apps/
 │   ├─ android/
 │   │   └─ assets/
 │   │        ├─ logo.png
 │   │        └─ logo.svg
 │   ├─ ios/
 │   │   └─ assets/
 │   │        ├─ logo.png
 │   │        └─ logo.svg
 │   ├─ site/
 │   │   ├─ assets/
 │   │   │    ├─ logo.png
 │   │   │    └─ logo.svg
 │   │   ├─ scripts/
 │   │   │    └─ vercel-ignore.sh
 │   │   ├─ index.html
 │   │   ├─ site.css
 │   │   ├─ site.js
 │   │   └─ vercel.json
 │   ├─ web/
 │   │   ├─ assets/
 │   │   │    ├─ logo.png
 │   │   │    └─ logo.svg
 │   │   ├─ scripts/
 │   │   │    └─ vercel-ignore.sh
 │   │   ├─ index.html
 │   │   ├─ web.css
 │   │   ├─ web.js
 │   │   └─ vercel.json
 │   └─ wechat/
 │       └─ （空，预留）
 │
 ├─ assets/
 │   ├─ logo.png
 │   └─ logo.svg
 │
 ├─ docs/
 │   ├─ changelog.md
 │   ├─ roadmap.md
 │   ├─ journal.md
 │   ├─ vercel.md
 │   ├─ structure.md
 │   └─ conventions.md
 │
 ├─ LICENSE
 └─ README.md
```

# XGit 项目结构说明

本文件记录 XGit 的目录结构及其约定。

---

## 1. apps 目录
放置所有面向用户的应用子项目：

- **android/**  
  Android 端应用，包含自身的 `assets/`。

- **ios/**  
  iOS 端应用，包含自身的 `assets/`。

- **site/**  
  官网 (Landing Page)，包含：
  - `index.html`  
  - `site.css`  
  - `site.js`  
  - `assets/` (仅站点专用资源)
  - `scripts/` (vercel部署排除更新脚本)

- **web/**  
  网页版 Git 编辑器 (核心交互 App)，包含：
  - `index.html`  
  - `web.css`  
  - `web.js`  
  - `assets/`
  - `scripts/` (vercel部署排除更新脚本)  

- **wechat/**  
  微信小程序 (预留目录)。

---

## 2. assets 目录
放置全局公用的资源文件，例如：
- `logo.png`  
- `logo.svg`

说明：  
- 若某端需要定制资源，可在对应 `apps/*/assets/` 下放置。  
- 全局资源尽量保持少量且稳定。

---

## 3. docs 目录
项目相关的文档记录：

- `changelog.md` —— 更新日志  
- `roadmap.md` —— 发展路线  
- `journal.md` —— 开发日记 / 碎片想法  
- `vercel.md` —— 部署与环境配置说明  
- `structure.md` —— 本文件，项目结构说明  
- `conventions.md` —— 命名、分支、提交信息规范  

---

## 4. 根目录文件
- `README.md` —— 项目说明与快速上手  
- `LICENSE` —— 开源协议 (MIT)