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
    pleaseSignIn: '请先登录',
    noRepos: '没有可显示的仓库'
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
    pleaseSignIn: 'Please sign in first',
    noRepos: 'No repositories to show'
  }
};

function t(key){
  return (dict[lang] && dict[lang][key]) ?? (dict.zh[key] ?? key);
}
function applyI18n(){
  $$('[data-i18n]').forEach(el => {
    const key = el.getAttribute('data-i18n');
    el.textContent = t(key);
  });
  document.documentElement.lang = lang === 'zh' ? 'zh' : 'en';
}

/* ---------- auth/API ---------- */
const LS_TOKEN = 'xgit_token';

async function fetchJson(url, token){
  // 兼容经典 token 与 fine-grained（有的更偏好 Bearer）
  const baseHeaders = {'Accept':'application/vnd.github+json'};
  const tryOrder = token ? [
    { Authorization: `Bearer ${token}`, ...baseHeaders },
    { Authorization: `token ${token}`,  ...baseHeaders },
  ] : [ baseHeaders ];

  let lastErr;
  for (const headers of tryOrder){
    try{
      const r = await fetch(url, { headers });
      if (!r.ok) throw new Error('HTTP '+r.status);
      return await r.json();
    }catch(e){ lastErr = e; }
  }
  throw lastErr || new Error('request failed');
}

async function apiMe(){ 
  const tk = localStorage.getItem(LS_TOKEN);
  if(!tk) throw new Error('no token');
  return fetchJson('https://api.github.com/user', tk);
}

async function apiRepos({ page=1, per_page=100 }={}){
  const tk = localStorage.getItem(LS_TOKEN);
  if(!tk) throw new Error('no token');
  // visibility+affiliation 尽量覆盖自有/协作/组织
  const qs = new URLSearchParams({
    per_page: String(per_page),
    page: String(page),
    sort: 'updated',
    affiliation: 'owner,collaborator,organization_member',
    visibility: 'all'
  }).toString();
  return fetchJson(`https://api.github.com/user/repos?${qs}`, tk);
}

/* ---------- UI 状态 ---------- */
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
    // 清空仓库列表
    $('#repoList').classList.add('hidden');
    $('#repoList').innerHTML = '';
  }
}

/* ---------- 登录流 ---------- */
async function validateToken(){
  const token = localStorage.getItem(LS_TOKEN);
  if(!token) return null;
  try{ return await apiMe(); }catch{ return null; }
}

async function signInFlow(){
  const token = prompt('GitHub Personal Access Token（建议仅勾选 repo / 或 Fine-grained Read）:');
  if(!token) return;
  localStorage.setItem(LS_TOKEN, token.trim());
  const me = await validateToken();
  if(me){
    setSignedUI(me);
    await loadRepos(); // ★ 登录成功后立刻加载仓库
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
  if(me) await loadRepos(); // ★ 刷新时也重载仓库
}

/* ---------- 仓库列表：实现渲染 ---------- */
async function loadRepos(){
  const token = localStorage.getItem(LS_TOKEN);
  if(!token){ return; }

  // 简单加载（前 100 个）
  let repos = [];
  try{
    repos = await apiRepos({ per_page: 100, page: 1 });
  }catch(e){
    console.warn(e);
    $('#repoList').classList.remove('hidden');
    $('#repoList').innerHTML = `<li>${esc(lang==='zh'?'加载仓库失败':'Failed to load repositories')}</li>`;
    return;
  }

  // 渲染
  $('#repoList').classList.remove('hidden');
  if(!repos || repos.length===0){
    $('#repoList').innerHTML = `<li>${esc(t('noRepos'))}</li>`;
    return;
  }

  const html = repos.map(r=>{
    const privacy = r.private ? '🔒' : '🌐';
    const full = `${r.owner?.login || ''}/${r.name}`;
    const langTag = r.language ? `<span class="tag">${esc(r.language)}</span>` : '';
    const br = r.default_branch || 'main';
    const updated = r.pushed_at ? new Date(r.pushed_at).toLocaleString() : '';
    return `<li class="repo" data-owner="${esc(r.owner.login)}" data-repo="${esc(r.name)}" data-branch="${esc(br)}">
      <div class="row">
        <div class="left">
          <div class="name">${privacy} ${esc(full)}</div>
          <div class="meta">
            ${langTag}
            <span class="tag">branch: ${esc(br)}</span>
            <span class="tag">updated: ${esc(updated)}</span>
          </div>
        </div>
        <div class="right">
          <a class="chip" href="${esc(r.html_url)}" target="_blank" rel="noreferrer">GitHub</a>
        </div>
      </div>
    </li>`;
  }).join('');

  $('#repoList').innerHTML = html;

  // （预留）点击进入仓库：下一步接文件树
  $$('#repoList .repo').forEach(li=>{
    li.onclick = ()=>{
      const owner = li.dataset.owner;
      const repo  = li.dataset.repo;
      const br    = li.dataset.branch;
      console.log('open repo:', owner, repo, br);
      alert((lang==='zh'?'即将打开仓库：':'Open repo: ') + `${owner}/${repo} (${br})`);
      // TODO: 下一步接入：openRepo(owner, repo, br);
    };
  });
}

/* ---------- 绑定 & 启动 ---------- */
function bind(){
  // 登录按钮（顶部与欢迎卡片共用逻辑）
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
  $('#langZh').onclick = ()=>{ lang='zh'; localStorage.setItem('xgit_lang',lang); applyI18n(); };
  $('#langEn').onclick = ()=>{ lang='en'; localStorage.setItem('xgit_lang',lang); applyI18n(); };
}

async function boot(){
  // 恢复语言
  const saved = localStorage.getItem('xgit_lang');
  if(saved) lang = saved;
  applyI18n();
  bind();

  // 首次渲染登录状态 & 仓库
  const me = await validateToken();
  setSignedUI(me);
  if(me) await loadRepos();
}

boot();