// Token login (PAT) + local storage helpers
const LS_PAT = 'xgit_pat';

export function getStoredToken(){
  const t = localStorage.getItem(LS_PAT);
  return t ? { access_token: t } : null;
}

export function setStoredToken(token){
  localStorage.setItem(LS_PAT, token);
}

export function clearToken(){
  localStorage.removeItem(LS_PAT);
}

// 预留：设备码登录占位（需服务端代理 CORS）
export async function startDeviceLogin(){ throw new Error('Device Flow 需后端代理，暂不支持'); }
export async function pollForToken(){ throw new Error('Device Flow 需后端代理，暂不支持'); }