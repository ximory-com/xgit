/* ---------- utils ---------- */
const $ = (s, el=document) => el.querySelector(s);
const $$ = (s, el=document) => Array.from(el.querySelectorAll(s));
const esc = (s) => String(s ?? '').replace(/[&<>]/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));

/* ---------- i18n (fallback to zh) ---------- */
let lang = (localStorage.getItem('xgit_lang') || (navigator.language||'zh').toLowerCase().startsWith('zh') ? 'zh' : 'en');

const dict = {
  zh: {
    signIn: '用 Token 登录',
    signOut: '退出登录',
    refresh: '刷新',
    backToSite: '返回官网',
    welcomeTitle: '欢迎使用 XGit Web',
    welcomeSub: '随时随地，轻松管理你的 GitHub 仓库。',
    signedIn: '已登录',
    repos: '仓库',
    pleaseSignIn: '请先登录'
  },
  en: {
    signIn: 'Sign in with Token',
    signOut: 'Sign out',
    refresh: 'Refresh',
    backToSite: 'Back to Site',
    welcomeTitle: 'Welcome to XGit Web',
    welcomeSub: 'Manage your GitHub repos on the go.',
    signedIn: 'Signed in',
    repos: 'Repositories',
    pleaseSignIn: 'Please sign in first'
  }
};

function t(key){
  // 优先当前语言，找不到回退到中文，再回退 key
  return (dict[lang] && dict[lang][key]) ?? (dict.zh[key] ?? key);
}

function applyI18n(){
  $$('[data-i18n]').forEach(el => {
    const key = el.getAttribute('data-i18n');
    el.textContent = t(key);
  });
  document.documentElement.lang = lang === 'zh' ? 'zh' : 'en';
}

/* ---------- auth ---------- */
const LS_TOKEN = 'xgit_token';

async function api(path){
  const token = localStorage.getItem(LS_TOKEN);
  const res = await fetch(`https://api.github.com${path}`, {
    headers: token ? { 'Authorization': `token ${token}` } : {}
  });
  if(!res.ok) throw new Error(`${res.status}`);
  return res.json();
}

async function validateToken(){
  const token = localStorage.getItem(LS_TOKEN);
  if(!token) return null;
  try{
    const me = await api('/user');
    return me;
  }catch(e){
    console.warn('token invalid', e);
    return null;
  }
}

function setSignedUI(me){
  if(me){
    $('#userBox').classList.remove('hidden');
    $('#repoEmpty').classList.add('hidden');
    $('#userName').textContent = `${me.login}`;
    $('#userAvatar').src = `${me.avatar_url}&s=80`;
    $('#btnSign').textContent = t('signOut');
    $('#btnSign2').textContent = t('signOut');
    $('#btnSign').dataset.mode = 'out';
    $('#btnSign2').dataset.mode = 'out';
  }else{
    $('#userBox').classList.add('hidden');
    $('#repoEmpty').classList.remove('hidden');
    $('#btnSign').textContent = t('signIn');
    $('#btnSign2').textContent = t('signIn');
    delete $('#btnSign').dataset.mode;
    delete $('#btnSign2').dataset.mode;
    $('#userAvatar').removeAttribute('src');
    $('#userName').textContent = '-';
  }
}

/* ---------- events ---------- */
async function signInFlow(){
  const token = prompt('GitHub Personal Access Token (建议只勾选 repo 权限 / Fine-grained 也可)：');
  if(!token) return;
  localStorage.setItem(LS_TOKEN, token.trim());
  const me = await validateToken();
  if(me){
    setSignedUI(me);
    // 预留：后续加载仓库列表
    // loadRepos();
    alert(lang==='zh'?'登录成功':'Signed in');
  }else{
    localStorage.removeItem(LS_TOKEN);
    alert(lang==='zh'?'Token 无效':'Invalid token');
  }
}

async function signOutFlow(){
  localStorage.removeItem(LS_TOKEN);
  setSignedUI(null);
}

async function refreshFlow(){
  const me = await validateToken();
  setSignedUI(me);
}

/* ---------- repos (占位) ---------- */
async function loadRepos(){
  const token = localStorage.getItem(LS_TOKEN);
  if(!token){ return; }
  // 先给空列表占位，后续会实现真正的拉取
  $('#repoList').classList.remove('hidden');
  $('#repoList').innerHTML = '';
}

/* ---------- boot ---------- */
function bind(){
  // 多个“登录”按钮共用一套逻辑
  ['btnSign','btnSign2'].forEach(id=>{
    $('#'+id).onclick = async (e)=>{
      if(e.currentTarget.dataset.mode === 'out'){ await signOutFlow(); }
      else { await signInFlow(); }
    };
  });
  $('#btnSignOut').onclick = signOutFlow;
  $('#btnRefresh').onclick = refreshFlow;
  $('#repoReload').onclick = loadRepos;

  // 语言切换
  $('#langZh').onclick = ()=>{ lang='zh'; localStorage.setItem('xgit_lang',lang); applyI18n(); setTimeout(refreshFlow,0); };
  $('#langEn').onclick = ()=>{ lang='en'; localStorage.setItem('xgit_lang',lang); applyI18n(); setTimeout(refreshFlow,0); };

  // “返回官网”在同一窗口打开（保持当前 a 行为即可）
  $('#backToSite').onclick = ()=>{ /* 默认同窗，无需 window.open */ };
}

async function boot(){
  // 恢复语言
  const saved = localStorage.getItem('xgit_lang');
  if(saved) lang = saved;

  applyI18n();
  bind();

  // 首次渲染登录状态
  const me = await validateToken();
  setSignedUI(me);
  if(me) await loadRepos();
}

boot();