# Vercel 部署说明

本文件记录 XGit 在 Vercel 上的部署方法与规范。

---

## 1. 项目结构
apps/
├─ site/   → 官网
├─ web/    → Web App
├─ ios/    → iOS 相关资源
├─ android/→ Android 相关资源
├─ wechat/ → 微信小程序资源
assets/      → 全局 logo 与公共资源
docs/        → 文档目录

---

## 2. 部署策略
- 官网 (site)：部署到 **主域** `xgit.ximory.com`
- Web App (web)：部署到 **子域** `app.xgit.ximory.com`

---

## 3. 配置步骤
### 3.1 创建 Vercel 项目
1. 登录 [Vercel](https://vercel.com/)
2. 新建 Project，选择 GitHub 仓库 **ximory/xgit**
3. 框架选择 **Other**
4. 根目录设置为仓库根路径

---

### 3.2 官网 (site) 配置
- **Root Directory**: `apps/site`
- **Framework Preset**: Other
- **Build Command**: 空
- **Output Directory**: `.`

域名绑定：`xgit.ximory.com`

---

### 3.3 Web App 配置
- **Root Directory**: `apps/web`
- **Framework Preset**: Other
- **Build Command**: 空
- **Output Directory**: `.`

域名绑定：`app.xgit.ximory.com`

---

## 4. 环境变量
当前版本 (v0.1.0) 无需环境变量。  
后续如需 GitHub OAuth，可在 Vercel `Settings → Environment Variables` 配置：
- `GITHUB_CLIENT_ID`
- `GITHUB_CLIENT_SECRET`

---

## 5. 注意事项
- `.DS_Store` 等本地文件已在 `.gitignore` 中忽略。
- 每次修改后，推送到 GitHub 即触发自动部署。
- 如需回滚，可在 Vercel **Deployments** 页面选择历史版本。

---