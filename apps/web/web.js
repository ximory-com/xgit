// ========== utils ==========
const $  = (s, el=document) => el.querySelector(s);
const $$ = (s, el=document) => Array.from(el.querySelectorAll(s));
const esc = (s) => String(s ?? '').replace(/[&<>]/g, c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));

// ========== i18n (fallback zh) ==========
let lang = (localStorage.getItem('xgit_lang') || '').trim();
if (!lang) {
  const nav = (navigator.language || 'zh').toLowerCase();
  lang = nav.startsWith('zh') ? 'zh' : 'en';
  localStorage.setItem('xgit_lang', lang);
}
const dict = {
  zh:{ signIn:'用 Token 登录', signOut:'退出登录', refresh:'刷新', backToSite:'返回官网',
       welcomeTitle:'欢迎使用 XGit Web', welcomeSub:'随时随地，轻松管理你的 GitHub 仓库。',
       signedIn:'已登录', repos:'仓库', pleaseSignIn:'请先登录', backToList:'返回列表',
       downloadZip:'下载 ZIP', readme:'README（节选）' },
  en:{ signIn:'Sign in with Token', signOut:'Sign out', refresh:'Refresh', backToSite:'Back to Site',
       welcomeTitle:'Welcome to XGit Web', welcomeSub:'Manage your GitHub repos on the go.',
       signedIn:'Signed in', repos:'Repositories', pleaseSignIn:'Please sign in first', backToList:'Back to list',
       downloadZip:'Download ZIP', readme:'README (snippet)' }
};
function t(key){ return (dict[lang] && dict[lang][key]) ?? (dict.zh[key] ?? key); }
function applyI18n(){
  $$('[data-i18n]').forEach(el=> el.textContent = t(el.getAttribute('data-i18n')));
  document.documentElement.lang = lang === 'zh' ? 'zh' : 'en';
}

// ========== auth/API ==========
const LS_TOKEN = 'xgit_token';
const GH = 'https://api.github.com';

function ghHeaders(token, extra={}){
  return { 'Accept':'application/vnd.github+json', 'Authorization': `Bearer ${token}`, ...extra };
}
async function fetchJSON(url, token){
  const r = await fetch(url, { headers: token ? ghHeaders(token) : {'Accept':'application/vnd.github+json'} });
  if (!r.ok) throw new Error('HTTP '+r.status);
  return await r.json();
}
async function apiMe(){
  const tk = localStorage.getItem(LS_TOKEN); if(!tk) throw new Error('no token');
  return fetchJSON(`${GH}/user`, tk);
}
async function apiRepos({ page=1, per_page=100 }={}){
  const tk = localStorage.getItem(LS_TOKEN); if(!tk) throw new Error('no token');
  const qs = new URLSearchParams({
    per_page:String(per_page), page:String(page), sort:'updated',
    affiliation:'owner,collaborator,organization_member', visibility:'all'
  }).toString();
  return fetchJSON(`${GH}/user/repos?${qs}`, tk);
}
async function apiRepo(owner, repo){
  const tk = localStorage.getItem(LS_TOKEN); if(!tk) throw new Error('no token');
  return fetchJSON(`${GH}/repos/${owner}/${repo}`, tk);
}
async function apiReadme(owner, repo, ref){
  const tk = localStorage.getItem(LS_TOKEN); if(!tk) throw new Error('no token');
  const r = await fetch(`${GH}/repos/${owner}/${repo}/readme${ref?`?ref=${encodeURIComponent(ref)}`:''}`, { headers: ghHeaders(tk) });
  if (!r.ok) return null;
  const j = await r.json();
  if (!j || !j.content) return null;
  try {
    const txt = decodeURIComponent(escape(atob(j.content)));
    return txt;
  } catch { return null; }
}

// ========== UI 状态 ==========
function setSignedUI(me){
  const signed = !!me;
  $('#btnSign').textContent  = signed ? t('signOut') : t('signIn');
  $('#btnSign2').textContent = signed ? t('signOut') : t('signIn');
  $('#btnSign').dataset.mode = signed ? 'out' : 'in';
  $('#btnSign2').dataset.mode = signed ? 'out' : 'in';

  if (signed){
    $('#userAvatar')?.setAttribute('src', `${me.avatar_url}&s=40`);
    $('#userAvatarMini')?.setAttribute('src', `${me.avatar_url}&s=28`);
    $('#userName').textContent = me.login;
    $('#userNameMini').textContent = me.login;
    $('#userBox').classList.remove('hidden');
    $('#repoEmpty').classList.add('hidden');
  }else{
    $('#userAvatar')?.removeAttribute('src');
    $('#userAvatarMini')?.removeAttribute('src');
    $('#userName').textContent = '-';
    $('#userNameMini').textContent = '-';
    $('#userBox').classList.add('hidden');
    $('#repoEmpty').classList.remove('hidden');
    $('#repoList').classList.add('hidden');
    $('#repoList').innerHTML = '';
    // 回到列表视图
    showListView();
  }
}
async function validateToken(){
  const tk = localStorage.getItem(LS_TOKEN); if(!tk) return null;
  try{ return await apiMe(); }catch{ return null; }
}

// ========== 仓库列表渲染 ==========
async function loadRepos(){
  const token = localStorage.getItem(LS_TOKEN);
  if(!token){ return; }
  let repos = [];
  try{
    repos = await apiRepos({ per_page: 50, page: 1 });
  }catch(e){
    console.warn(e);
    $('#repoList').classList.remove('hidden');
    $('#repoList').innerHTML = `<li class="empty">${esc(lang==='zh'?'加载仓库失败':'Failed to load repositories')}</li>`;
    return;
  }
  $('#repoList').classList.remove('hidden');
  if(!repos || repos.length===0){
    $('#repoList').innerHTML = `<li class="empty">${esc(t('noRepos')|| (lang==='zh'?'没有可显示的仓库':'No repositories'))}</li>`;
    return;
  }

  const html = repos.map(r=>{
    const privacy = r.private ? '🔒' : '🌐';
    const full = `${r.owner?.login || ''}/${r.name}`;
    const br = r.default_branch || 'main';
    const updated = r.pushed_at ? new Date(r.pushed_at).toLocaleString() : '';
    const langTag = r.language ? `<span class="tag">${esc(r.language)}</span>` : '';
    const stars = r.stargazers_count ?? 0;
    const forks = r.forks_count ?? 0;
    const issues = r.open_issues_count ?? 0;
    const desc = r.description ? `<div class="desc">${esc(r.description)}</div>` : '';

    return `<li class="repo" data-owner="${esc(r.owner.login)}" data-repo="${esc(r.name)}" data-branch="${esc(br)}" data-url="${esc(r.html_url)}">
      <div class="row">
        <div class="left">
          <div class="name">${privacy} ${esc(full)}</div>
          ${desc}
          <div class="meta">
            ${langTag}
            <span class="tag">branch: ${esc(br)}</span>
            <span class="tag">updated: ${esc(updated)}</span>
            <span class="tag">★ ${stars}</span>
            <span class="tag">⑂ ${forks}</span>
            <span class="tag">● ${issues}</span>
          </div>
        </div>
        <div class="right">
          <a class="chip open" href="${esc(r.html_url)}" target="_blank" rel="noreferrer">GitHub</a>
          <div class="more">
            <button class="more-btn">…</button>
            <div class="more-menu">
              <div class="item" data-act="open">Open on GitHub</div>
              <div class="item" data-act="clone">Copy clone URL (SSH)</div>
              <div class="item" data-act="name">Copy owner/name</div>
              <div class="item" data-act="zip">Download ZIP</div>
            </div>
          </div>
        </div>
      </div>
    </li>`;
  }).join('');
  $('#repoList').innerHTML = html;

  // 绑定点击进入仓库
  $$('#repoList .repo').forEach(li=>{
    li.addEventListener('click', (e)=>{
      // 如果点的是更多菜单或链接，不进入仓库页
      if (e.target.closest('.more') || e.target.closest('a')) return;
      openRepoPage(li.dataset.owner, li.dataset.repo, li.dataset.branch, li.dataset.url);
    });
    // 更多菜单
    const more = li.querySelector('.more');
    const btn  = li.querySelector('.more-btn');
    const menu = li.querySelector('.more-menu');
    btn?.addEventListener('click', (e)=>{
      e.stopPropagation();
      more.classList.toggle('open');
    });
    menu?.addEventListener('click', (e)=>{
      e.stopPropagation();
      const act = e.target.closest('.item')?.dataset.act;
      if (!act) return;
      const owner = li.dataset.owner, repo = li.dataset.repo, br = li.dataset.branch;
      const url = li.dataset.url;
      switch(act){
        case 'open':
          window.open(url, '_blank'); break;
        case 'clone': {
          const ssh = `git@github.com:${owner}/${repo}.git`;
          navigator.clipboard?.writeText(ssh);
          alert((lang==='zh'?'已复制：':'Copied: ')+ssh);
          break;
        }
        case 'name': {
          const name = `${owner}/${repo}`;
          navigator.clipboard?.writeText(name);
          alert((lang==='zh'?'已复制：':'Copied: ')+name);
          break;
        }
        case 'zip': {
          const zip = `https://github.com/${owner}/${repo}/archive/refs/heads/${encodeURIComponent(br)}.zip`;
          window.open(zip, '_blank');
          break;
        }
      }
      more.classList.remove('open');
    });
    document.addEventListener('click', ()=> more.classList.remove('open'));
  });
}

// ========== 仓库详情页 ==========
function showRepoView(){ $('#repoPage').classList.remove('hidden'); }
function showListView(){ $('#repoPage').classList.add('hidden'); }
$('#repoBack')?.addEventListener('click', ()=> showListView());

async function openRepoPage(owner, repo, br, htmlUrl){
  showRepoView();
  $('#repoCrumbOwner').textContent = owner;
  $('#repoCrumbName').textContent = repo;
  $('#repoTitle').textContent = `${owner}/${repo}`;
  $('#repoBranch').textContent = `branch: ${br}`;
  $('#repoOnGitHub').setAttribute('href', htmlUrl);
  $('#repoZip').setAttribute('href', `https://github.com/${owner}/${repo}/archive/refs/heads/${encodeURIComponent(br)}.zip`);
  $('#repoReadme').textContent = '…';

  try{
    const r = await apiRepo(owner, repo);
    const updated = r.pushed_at ? new Date(r.pushed_at).toLocaleString() : '-';
    $('#repoUpdated').textContent = `updated: ${updated}`;
    $('#repoLang').textContent = r.language || '-';
    $('#repoStars').textContent = `★ ${r.stargazers_count ?? 0}`;
    $('#repoForks').textContent = `⑂ ${r.forks_count ?? 0}`;
    $('#repoIssues').textContent = `● ${r.open_issues_count ?? 0}`;
  }catch(e){
    console.warn(e);
  }

  try{
    const txt = await apiReadme(owner, repo, br);
    if (txt){
      const cut = txt.slice(0, 1200); // 片段
      $('#repoReadme').textContent = cut;
    }else{
      $('#repoReadme').textContent = lang==='zh'?'未找到 README':'README not found';
    }
  }catch{
    $('#repoReadme').textContent = lang==='zh'?'读取 README 失败':'Failed to load README';
  }
}

// ========== 登录流 ==========
async function signInFlow(){
  const token = prompt(lang==='zh'?'输入 GitHub Token（建议 fine-grained / repo read）':'Paste your GitHub Token');
  if(!token) return;
  localStorage.setItem(LS_TOKEN, token.trim());
  const me = await validateToken();
  if (me){
    setSignedUI(me);
    await loadRepos();
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
  if (me) await loadRepos();
}

// ========== 绑定 & 启动 ==========
function bind(){
  // 登录/退出（顶部与欢迎卡片按钮共享逻辑）
  ['btnSign','btnSign2'].forEach(id=>{
    const el = $('#'+id);
    el && el.addEventListener('click', async (e)=>{
      if (el.dataset.mode === 'out') await signOutFlow();
      else await signInFlow();
    });
  });
  $('#btnSignOut')?.addEventListener('click', signOutFlow);
  $('#btnRefresh')?.addEventListener('click', refreshFlow);
  $('#repoReload')?.addEventListener('click', loadRepos);

  // 语言
  $('#langZh')?.addEventListener('click', ()=>{ lang='zh'; localStorage.setItem('xgit_lang',lang); applyI18n(); });
  $('#langEn')?.addEventListener('click', ()=>{ lang='en'; localStorage.setItem('xgit_lang',lang); applyI18n(); });
}

async function boot(){
  applyI18n();
  bind();
  const me = await validateToken();
  setSignedUI(me);
  if (me) await loadRepos();
}

document.addEventListener('DOMContentLoaded', boot);
