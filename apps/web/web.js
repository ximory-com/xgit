/* ---------- utils ---------- */
const $ = (s, el=document)=> el.querySelector(s);
const $$ = (s, el=document)=> Array.from(el.querySelectorAll(s));
const esc = s => String(s??'').replace(/[&<>]/g, c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));

/* ---------- i18n (fallback zh) ---------- */
let lang = (localStorage.getItem('xgit_lang') || ((navigator.language||'zh').toLowerCase().startsWith('zh') ? 'zh' : 'en'));
const dict = {
  zh:{signIn:'用 Token 登录',signOut:'退出登录',refresh:'刷新',backToSite:'返回官网',welcomeTitle:'欢迎使用 XGit Web',welcomeSub:'随时随地，轻松管理你的 GitHub 仓库。',signedIn:'已登录',repos:'仓库',backToList:'返回列表',pleaseSignIn:'请先登录',noRepos:'没有可显示的仓库'},
  en:{signIn:'Sign in with Token',signOut:'Sign out',refresh:'Refresh',backToSite:'Back to Site',welcomeTitle:'Welcome to XGit Web',welcomeSub:'Manage your GitHub repos on the go.',signedIn:'Signed in',repos:'Repositories',backToList:'Back to list',pleaseSignIn:'Please sign in',noRepos:'No repositories to show'}
};
function t(k){ return (dict[lang]&&dict[lang][k]) ?? (dict.zh[k]??k); }
function applyI18n(){
  $$('[data-i18n]').forEach(el=> el.textContent = t(el.getAttribute('data-i18n')));
  document.documentElement.lang = lang==='zh'?'zh':'en';
}

/* ---------- auth/API ---------- */
const LS_TOKEN='xgit_token';
const GH = 'https://api.github.com';

async function fetchJson(url, token){
  const base={'Accept':'application/vnd.github+json'};
  const tries = token ? [
    {...base, Authorization:`Bearer ${token}`},
    {...base, Authorization:`token ${token}`},
  ] : [base];
  let last;
  for(const headers of tries){
    try{
      const r = await fetch(url,{headers});
      if(!r.ok) throw new Error('HTTP '+r.status);
      return await r.json();
    }catch(e){ last=e; }
  }
  throw last||new Error('request failed');
}

async function apiMe(){
  const tk = localStorage.getItem(LS_TOKEN); if(!tk) throw new Error('no token');
  return fetchJson(`${GH}/user`, tk);
}
async function apiRepos({page=1,per_page=100}={}){
  const tk = localStorage.getItem(LS_TOKEN); if(!tk) throw new Error('no token');
  const qs = new URLSearchParams({per_page:String(per_page),page:String(page),sort:'updated',affiliation:'owner,collaborator,organization_member',visibility:'all'}).toString();
  return fetchJson(`${GH}/user/repos?${qs}`, tk);
}
async function apiReadme(owner, repo, ref){
  const tk = localStorage.getItem(LS_TOKEN); if(!tk) throw new Error('no token');
  const data = await fetchJson(`${GH}/repos/${owner}/${repo}/readme${ref?`?ref=${encodeURIComponent(ref)}`:''}`, tk);
  if(!data || !data.content) return null;
  const txt = atob(data.content.replace(/\n/g,''));
  return txt;
}

/* ---------- Markdown (tiny) ---------- */
function mdEscape(s){ return String(s).replace(/[&<>]/g, m=>({ '&':'&amp;','<':'&lt;','>':'&gt;' }[m])); }
function mdToHtml(md){
  if(!md) return '';
  // normalize line endings
  md = md.replace(/\r\n?/g,'\n');

  // fenced code blocks ```lang\n...\n```
  md = md.replace(/```([\s\S]*?)```/g, (m, code)=> `<pre><code>${mdEscape(code.trim())}</code></pre>`);

  // headings
  md = md.replace(/^(#{1,3})[ \t]+(.+)$/gm, (m, hashes, text)=> {
    const h = hashes.length;
    return `<h${h}>${mdEscape(text.trim())}</h${h}>`;
  });

  // lists (very naive)
  md = md.replace(/^(?:-|\*) (.+)$/gm, (m, item)=> `<ul><li>${mdInline(item.trim())}</li></ul>`);
  md = md.replace(/^\d+\. (.+)$/gm, (m, item)=> `<ol><li>${mdInline(item.trim())}</li></ol>`);

  // paragraphs: split by blank lines, wrap if not already HTML block
  const blocks = md.split(/\n{2,}/).map(b=>{
    if (/^<(h\d|ul|ol|pre|blockquote)/.test(b.trim())) return b;
    return `<p>${mdInline(b.trim())}</p>`;
  });
  return blocks.join('\n');
}
function mdInline(s){
  if(!s) return '';
  // inline code
  s = s.replace(/`([^`]+)`/g, (m, c)=> `<code>${mdEscape(c)}</code>`);
  // bold **text**
  s = s.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>');
  // italics *text*
  s = s.replace(/\*([^*]+)\*/g, '<em>$1</em>');
  // links [text](url)
  s = s.replace(/\[([^\]]+)\]\((https?:\/\/[^\s)]+)\)/g, (m, text, url)=> `<a href="${url}" target="_blank" rel="noreferrer">${mdEscape(text)}</a>`);
  return s;
}

/* ---------- UI State ---------- */
function setSignedUI(me){
  if(me){
    $('#userBox').classList.remove('hidden');
    $('#repoEmpty').classList.add('hidden');
    $('#userName').textContent = `${me.login}`;
    $('#userAvatar').src = `${me.avatar_url}&s=80`;
    $('#btnSign2').textContent = t('signOut');
    $('#btnSign2').dataset.mode = 'out';
  }else{
    $('#userBox').classList.add('hidden');
    $('#repoEmpty').classList.remove('hidden');
    $('#btnSign2').textContent = t('signIn');
    delete $('#btnSign2').dataset.mode;
    $('#userAvatar').removeAttribute('src');
    $('#userName').textContent = '-';
    $('#repoList').classList.add('hidden');
    $('#repoList').innerHTML = '';
    $('#repoView').classList.add('hidden');
  }
}

/* ---------- Auth flows ---------- */
async function validateToken(){
  const tk = localStorage.getItem(LS_TOKEN);
  if(!tk) return null;
  try{ return await apiMe(); }catch{ return null; }
}
async function signInFlow(){
  const token = prompt('GitHub Personal Access Token（建议仅勾选 repo / 或 Fine-grained Read）:');
  if(!token) return;
  localStorage.setItem(LS_TOKEN, token.trim());
  const me = await validateToken();
  if(me){
    setSignedUI(me);
    await loadRepos();
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
  if(me) await loadRepos();
}

/* ---------- Repo list & details ---------- */
let lastReposCache = []; // cache for details
async function loadRepos(){
  const tk = localStorage.getItem(LS_TOKEN); if(!tk){ return; }
  let repos = [];
  try{
    repos = await apiRepos({per_page:100,page:1});
    lastReposCache = repos;
  }catch(e){
    console.warn(e);
    $('#repoList').classList.remove('hidden');
    $('#repoList').innerHTML = `<li>${esc(lang==='zh'?'加载仓库失败':'Failed to load repositories')}</li>`;
    return;
  }
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
    const desc = r.description ? `<div class="meta">${esc(r.description)}</div>` : '';
    return `<li class="repo" data-owner="${esc(r.owner.login)}" data-repo="${esc(r.name)}" data-branch="${esc(br)}">
      <div class="row">
        <div class="left">
          <div class="name">${privacy} ${esc(full)}</div>
          ${desc}
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

  // bind click -> open repo
  $$('#repoList .repo').forEach(li=>{
    li.onclick = () => openRepo(li.dataset.owner, li.dataset.repo, li.dataset.branch);
  });
}

async function openRepo(owner, repo, branch){
  // find cached item
  const r = (lastReposCache||[]).find(x=> x.owner?.login===owner && x.name===repo) || {};
  $('#repoFullName').textContent = `${owner}/${repo}`;
  $('#repoDesc').textContent = r.description || '';
  $('#repoLang').textContent = r.language || '';
  $('#repoBranch').textContent = `branch: ${branch}`;
  $('#repoUpdated').textContent = r.pushed_at ? new Date(r.pushed_at).toLocaleString() : '';
  $('#repoStars').textContent = `★ ${r.stargazers_count ?? 0}`;
  $('#repoForks').textContent = `⑂ ${r.forks_count ?? 0}`;
  $('#repoIssues').textContent = `⚑ ${r.open_issues ?? 0}`;
  $('#repoGithub').href = r.html_url || `https://github.com/${owner}/${repo}`;
  $('#repoZip').href = `https://github.com/${owner}/${repo}/archive/refs/heads/${encodeURIComponent(branch)}.zip`;

  // fetch README
  $('#repoReadme').innerHTML = '<em>Loading README…</em>';
  try{
    const txt = await apiReadme(owner, repo, branch);
    const html = mdToHtml(txt);
    $('#repoReadme').innerHTML = html || '<em>No README</em>';
  }catch(e){
    console.warn(e);
    $('#repoReadme').innerHTML = '<em>Failed to load README</em>';
  }

  // show detail, ensure in view
  $('#repoView').classList.remove('hidden');
  $('#repoView').scrollIntoView({behavior:'smooth', block:'start'});

  // back to list
  $('#repoBack').onclick = ()=>{
    $('#repoView').classList.add('hidden');
    document.body.scrollIntoView({behavior:'smooth', block:'start'});
  };
}

/* ---------- bind & boot ---------- */
function bind(){
  // sign button in hero
  $('#btnSign2').onclick = async (e)=>{
    if(e.currentTarget.dataset.mode === 'out'){ await signOutFlow(); }
    else { await signInFlow(); }
  };
  $('#btnSignOut').onclick = signOutFlow;
  $('#btnRefresh').onclick = refreshFlow;
  $('#repoReload').onclick = loadRepos;

  // language
  $('#langZh').onclick = ()=>{ lang='zh'; localStorage.setItem('xgit_lang',lang); applyI18n(); };
  $('#langEn').onclick = ()=>{ lang='en'; localStorage.setItem('xgit_lang',lang); applyI18n(); };
}

async function boot(){
  applyI18n();
  bind();
  const me = await validateToken();
  setSignedUI(me);
  if(me) await loadRepos();
}
boot();
