/* ---------- utils ---------- */
const $ = (s, el=document)=>el.querySelector(s);
const $$ = (s, el=document)=>Array.from(el.querySelectorAll(s));
const esc = s => String(s??'').replace(/[&<>]/g, c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));

/* ---------- i18n (fallback to zh) ---------- */
let lang = (localStorage.getItem('xgit_lang') || ((navigator.language||'zh').toLowerCase().startsWith('zh') ? 'zh' : 'en'));
const dict = {
    signIn: '用 Token 登录', signOut: '退出登录', refresh: '刷新', backToSite:'返回官网', back:'返回',
    signIn: '用 Token 登录', signOut: '退出登录', refresh: '刷新', backToSite:'官网', back:'返回',
    signIn: '用 Token 登录', signOut: '退出登录', refresh: '刷新', backToSite:'返回官网', back:'返回',
    welcomeTitle:'欢迎使用 XGit Web', welcomeSub:'随时随地，轻松管理你的 GitHub 仓库。',
    signedIn:'已登录', repos:'仓库', pleaseSignIn:'请先登录', readme:'README',
    openGithub:'在 GitHub 打开', zip:'下载 ZIP', copyGit:'复制 Git URL', downloadZip:'下载 ZIP',
    noRepos:'暂无仓库'
  },
    signIn: 'Sign in with Token', signOut:'Sign out', refresh:'Refresh', backToSite:'Back to Site', back:'Back',
    signIn: 'Sign in with Token', signOut:'Sign out', refresh:'Refresh', backToSite:'Site', back:'Back',
    signIn: 'Sign in with Token', signOut:'Sign out', refresh:'Refresh', backToSite:'Back to Site', back:'Back',
    welcomeTitle:'Welcome to XGit Web', welcomeSub:'Manage your GitHub repos on the go.',
    signedIn:'Signed in', repos:'Repositories', pleaseSignIn:'Please sign in first', readme:'README',
    openGithub:'Open on GitHub', zip:'Download ZIP', copyGit:'Copy Git URL', downloadZip:'Download ZIP',
    noRepos:'No repositories found'
  }
};
function t(key){ return (dict[lang] && dict[lang][key]) ?? (dict['zh'][key] ?? key); }
function applyI18n(){
  $$('[data-i18n]').forEach(el=>{ el.textContent = t(el.getAttribute('data-i18n')); });
  document.documentElement.lang = (lang==='zh'?'zh':'en');
  
  // 更新语言按钮状态
  $('#langZh').classList.toggle('active', lang==='zh');
  $('#langEn').classList.toggle('active', lang==='en');
  
  // 保持登录状态的按钮文本
  const btnSign = $('#btnSign');
  const btnSign2 = $('#btnSign2');
  if(btnSign && btnSign.dataset.mode === 'out') {
    btnSign.textContent = t('signOut');
  } else if(btnSign) {
    btnSign.textContent = t('signIn');
  }
  if(btnSign2 && btnSign2.dataset.mode === 'out') {
    btnSign2.textContent = t('signOut');
  } else if(btnSign2) {
    btnSign2.textContent = t('signIn');
  }
}

/* ---------- auth/api ---------- */
const LS_TOKEN = 'xgit_token';
let currentRepo = null; // 当前查看的仓库信息

async function fetchJson(url, token){
  const base = {'Accept':'application/vnd.github+json'};
  const tries = token ? [
    { ...base, Authorization:`Bearer ${token}`},
    { ...base, Authorization:`token ${token}`},
  ] : [ base ];
  let lastErr;
  for (const headers of tries){
    try{
      const r = await fetch(url, { headers });
      if(!r.ok) throw new Error('HTTP '+r.status);
      return await r.json();
    }catch(e){ lastErr = e; }
  }
  throw lastErr || new Error('request failed');
}

async function apiMe(){
  const tk = localStorage.getItem(LS_TOKEN); if(!tk) throw new Error('no token');
  return fetchJson('https://api.github.com/user', tk);
}

async function apiRepos({page=1, per_page=100}={}){
  const tk = localStorage.getItem(LS_TOKEN); if(!tk) throw new Error('no token');
  const qs = new URLSearchParams({
    per_page:String(per_page), page:String(page), sort:'updated',
    affiliation:'owner,collaborator,organization_member', visibility:'all'
  }).toString();
  return fetchJson('https://api.github.com/user/repos?'+qs, tk);
}

async function apiReadme(owner, repo, ref){
  const tk = localStorage.getItem(LS_TOKEN); if(!tk) throw new Error('no token');
  const headers = {
    'Accept':'application/vnd.github.v3.raw',
    'Authorization': 'Bearer '+tk
  };
  const r = await fetch(`https://api.github.com/repos/${owner}/${repo}/readme?ref=${encodeURIComponent(ref)}`, { headers });
  if(!r.ok) return '';
  return await r.text();
}

/* ---------- state/ui ---------- */
  const repoCard = $('#repoListCard');
  if(me){
    $('#userBox').hidden = false;
    $('#repoEmpty').hidden = true;
    if(repoCard) repoCard.hidden = false; // 显示仓库区
    $('#userName').textContent = me.login;
    $('#userAvatar').src = me.avatar_url + '&s=80';
    
    // 设置登出按钮样式为橙色
    const btnSign = $('#btnSign');
    const btnSign2 = $('#btnSign2');
    if(btnSign) {
      btnSign.textContent = t('signOut');
      btnSign.dataset.mode = 'out';
      btnSign.className = 'btn warning'; // 橙色样式
    }
    if(btnSign2) {
      btnSign2.textContent = t('signOut');
      btnSign2.dataset.mode = 'out';
      btnSign2.className = 'btn warning'; // 橙色样式
    }
  }else{
    $('#userBox').hidden = true;
    $('#repoEmpty').hidden = false;
    if(repoCard) repoCard.hidden = true; // 隐藏仓库区
    $('#repoList').innerHTML='';
    $('#repoList').hidden = true;
    
    // 恢复登录按钮样式
    const btnSign = $('#btnSign');
    const btnSign2 = $('#btnSign2');
    if(btnSign) {
      btnSign.textContent = t('signIn');
      delete btnSign.dataset.mode;
      btnSign.className = 'btn danger';
    }
    if(btnSign2) {
      btnSign2.textContent = t('signIn');
      delete btnSign2.dataset.mode;
      btnSign2.className = 'btn primary';
    }
  }
}
function setSignedUI(me){
  const repoCard = $('#repoListCard');
  if(me){
    $('#userBox').hidden = false;
    $('#repoEmpty').hidden = true;
    if(repoCard) {
      repoCard.hidden = false; // 显示仓库区
      repoCard.classList.remove('hidden'); // 确保移除hidden类
    }
    $('#userName').textContent = me.login;
    $('#userAvatar').src = me.avatar_url + '&s=80';
    
    // 设置登出按钮样式为橙色
    const btnSign = $('#btnSign');
    const btnSign2 = $('#btnSign2');
    if(btnSign) {
      btnSign.textContent = t('signOut');
      btnSign.dataset.mode = 'out';
      btnSign.className = 'btn warning'; // 橙色样式
    }
    if(btnSign2) {
      btnSign2.textContent = t('signOut');
      btnSign2.dataset.mode = 'out';
      btnSign2.className = 'btn warning'; // 橙色样式
    }
  }else{
    $('#userBox').hidden = true;
    $('#repoEmpty').hidden = false;
    if(repoCard) {
      repoCard.hidden = true; // 隐藏仓库区
      repoCard.classList.add('hidden'); // 确保添加hidden类
    }
    $('#repoList').innerHTML='';
    $('#repoList').hidden = true;
    
    // 恢复登录按钮样式
    const btnSign = $('#btnSign');
    const btnSign2 = $('#btnSign2');
    if(btnSign) {
      btnSign.textContent = t('signIn');
      delete btnSign.dataset.mode;
      btnSign.className = 'btn danger';
    }
    if(btnSign2) {
      btnSign2.textContent = t('signIn');
      delete btnSign2.dataset.mode;
      btnSign2.className = 'btn primary';
    }
  }
}
  const repoCard = $('#repoListCard');
  if(me){
    $('#userBox').hidden = false;
    $('#repoEmpty').hidden = true;
    if(repoCard) repoCard.hidden = false; // 显示仓库区
    $('#userName').textContent = me.login;
    $('#userAvatar').src = me.avatar_url + '&s=80';
    
    // 设置登出按钮样式为橙色
    const btnSign = $('#btnSign');
    const btnSign2 = $('#btnSign2');
    if(btnSign) {
      btnSign.textContent = t('signOut');
      btnSign.dataset.mode = 'out';
      btnSign.className = 'btn warning'; // 橙色样式
    }
    if(btnSign2) {
      btnSign2.textContent = t('signOut');
      btnSign2.dataset.mode = 'out';
      btnSign2.className = 'btn warning'; // 橙色样式
    }
  }else{
    $('#userBox').hidden = true;
    $('#repoEmpty').hidden = false;
    if(repoCard) repoCard.hidden = true; // 隐藏仓库区
    $('#repoList').innerHTML='';
    $('#repoList').hidden = true;
    
    // 恢复登录按钮样式
    const btnSign = $('#btnSign');
    const btnSign2 = $('#btnSign2');
    if(btnSign) {
      btnSign.textContent = t('signIn');
      delete btnSign.dataset.mode;
      btnSign.className = 'btn danger';
    }
    if(btnSign2) {
      btnSign2.textContent = t('signIn');
      delete btnSign2.dataset.mode;
      btnSign2.className = 'btn primary';
    }
  }
}

/* ---------- sign flows ---------- */
async function validateToken(){
  const token = localStorage.getItem(LS_TOKEN); if(!token) return null;
  try{ return await apiMe(); }catch{ return null; }
}

  const token = prompt(lang==='zh'?'请输入 GitHub Token（建议 scope: repo）':'Paste a GitHub Token (scope: repo)');
  if(!token) return;
  localStorage.setItem(LS_TOKEN, token.trim());
  const me = await validateToken();
  if(me){ setSignedUI(me); await loadRepos(); alert(lang==='zh'?'登录成功':'Signed in'); }
  else { localStorage.removeItem(LS_TOKEN); alert(lang==='zh'?'Token 无效':'Invalid token'); }
}
async function signInFlow(){
  const token = prompt(lang==='zh'?'请输入 GitHub Token（建议 scope: repo）':'Paste a GitHub Token (scope: repo)');
  if(!token) return;
  localStorage.setItem(LS_TOKEN, token.trim());
  try{
    const me = await validateToken();
    if(me){ 
      setSignedUI(me); 
      await loadRepos(); 
      alert(lang==='zh'?'登录成功':'Signed in'); 
    } else { 
      localStorage.removeItem(LS_TOKEN); 
      alert(lang==='zh'?'Token 无效':'Invalid token'); 
    }
  }catch(e){
    localStorage.removeItem(LS_TOKEN);
    alert(lang==='zh'?'登录失败: '+e.message:'Login failed: '+e.message);
  }
}
  const token = prompt(lang==='zh'?'请输入 GitHub Token（建议 scope: repo）':'Paste a GitHub Token (scope: repo)');
  if(!token) return;
  localStorage.setItem(LS_TOKEN, token.trim());
  const me = await validateToken();
  if(me){ setSignedUI(me); await loadRepos(); alert(lang==='zh'?'登录成功':'Signed in'); }
  else { localStorage.removeItem(LS_TOKEN); alert(lang==='zh'?'Token 无效':'Invalid token'); }
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

/* ---------- markdown (very lite) ---------- */
function mdToHtml(md=''){
  md = md.replace(/</g,'&lt;').replace(/>/g,'&gt;');
  md = md.replace(/```([\s\S]*?)```/g, (_,code)=>'<pre><code>'+code.replace(/\n/g,'\n')+'</code></pre>');
  md = md.replace(/^###\s+(.*)$/gm,'<h3>$1</h3>')
         .replace(/^##\s+(.*)$/gm,'<h2>$1</h2>')
         .replace(/^#\s+(.*)$/gm,'<h1>$1</h1>');
  md = md.replace(/`([^`]+)`/g,'<code>$1</code>');
  md = md.replace(/\[([^\]]+)\]\(([^\)]+)\)/g,'<a href="$2" target="_blank" rel="noreferrer">$1</a>');
  md = md.replace(/^\s*[-*]\s+(.*)$/gm,'<li>$1</li>');
  md = md.replace(/(<li>.*<\/li>)(?![\s\S]*<li>)/g, '<ul>$1</ul>');
  md = md.replace(/(^|\n)([^<\n][^\n]*)(?=\n|$)/g, (m, p1, p2)=> p2.trim()? p1+'<p>'+p2+'</p>' : m);
  return md;
}

/* ---------- repos ---------- */
async function loadRepos(){
  const token = localStorage.getItem(LS_TOKEN); if(!token) return;
  
  $('#repoList').hidden = false;
  $('#repoEmpty').hidden = true;
  $('#repoList').innerHTML = '<li class="muted" style="padding:20px;text-align:center">Loading…</li>';
  
  let repos = [];
  try{ repos = await apiRepos({ per_page: 100, page: 1 }); }
  catch(e){ 
    $('#repoList').innerHTML='<li class="muted" style="padding:20px;text-align:center;color:#e74c3c">加载失败</li>'; 
    return; 
  }

  if(!Array.isArray(repos) || repos.length===0){
    $('#repoList').innerHTML = `<li class="muted" style="padding:20px;text-align:center">${t('noRepos')}</li>`;
    return;
  }

  const html = repos.map(r=>{
    const privacy = r.private ? '🔒' : '🌐';
    const full = `${r.owner?.login||''}/${r.name}`;
    const br = r.default_branch || 'main';
    const updated = r.pushed_at ? new Date(r.pushed_at).toLocaleString() : '';
    const desc = r.description ? esc(r.description) : '';
    const langTag = r.language ? `<span class="tag">${esc(r.language)}</span>` : '';
    return `<li class="repo" data-owner="${esc(r.owner.login)}" data-repo="${esc(r.name)}" data-branch="${esc(br)}" data-html="${esc(r.html_url)}">
      <div class="row">
        <div class="left">
          <div class="name">${privacy} ${esc(full)}</div>
          <div class="meta">
            ${langTag}
            <span class="tag">★ ${r.stargazers_count||0}</span>
            <span class="tag">⑂ ${r.forks_count||0}</span>
            <span class="tag">! ${r.open_issues_count||0}</span>
            <span class="tag">branch: ${esc(br)}</span>
            <span class="tag muted" style="font-size:10px">${esc(updated)}</span>
          </div>
          ${desc?`<div class="desc">${desc}</div>`:''}
        </div>
        <div class="right">
          <button class="btn ghost btn-more" style="position:relative">⋯</button>
        </div>
      </div>
      <div class="menu hidden">
        <button class="item open-gh">${t('openGithub')}</button>
        <button class="item dl-zip">${t('zip')}</button>
        <button class="item cp-git">${t('copyGit')}</button>
      </div>
    </li>`;
  }).join('');

  $('#repoList').innerHTML = html;

  $$('#repoList .repo').forEach(li=>{
    li.onclick = (e)=>{
      if(e.target.closest('.btn-more, .menu')) return;
      openRepoDetail(li.dataset.owner, li.dataset.repo, li.dataset.branch, li.dataset.html);
    };
    const menuBtn = li.querySelector('.btn-more');
    const menu = li.querySelector('.menu');
    menuBtn.onclick = (e)=>{
      e.stopPropagation();
      $$('.menu').forEach(m => m !== menu && m.classList.add('hidden'));
      menu.classList.toggle('hidden');
    };
    menu.querySelector('.open-gh').onclick = e=>{ e.stopPropagation(); window.open(li.dataset.html,'_blank'); };
    menu.querySelector('.dl-zip').onclick = e=>{
      e.stopPropagation();
      const zip = `${li.dataset.html}/archive/refs/heads/${li.dataset.branch}.zip`;
      window.open(zip,'_blank');
    };
    menu.querySelector('.cp-git').onclick = async e=>{
      e.stopPropagation();
      const url = `git@github.com:${li.dataset.owner}/${li.dataset.repo}.git`;
      try{ await navigator.clipboard.writeText(url); alert('Copied: '+url); }catch{ alert(url); }
    };
  });
  
  document.addEventListener('click', e => {
    if (!e.target.closest('.menu, .btn-more')) {
      $$('.menu').forEach(m => m.classList.add('hidden'));
    }
  });
}

async function openRepoDetail(owner, repo, branch, htmlUrl){
  currentRepo = { owner, repo, branch, htmlUrl };
  
  $('#repoDetailCard').hidden = false;
  $('#repoListCard').hidden = true;
  $('#welcomeCard').hidden = true;
  
  $('#repoFullname').textContent = `${owner}/${repo}`;
  $('#repoBranch').textContent = branch;
  $('#btnOpenGH').onclick = ()=> window.open(htmlUrl,'_blank');
  $('#btnZip').onclick = ()=> window.open(`${htmlUrl}/archive/refs/heads/${branch}.zip`,'_blank');

  $('#readmeBox').innerHTML = '<div class="muted">Loading README…</div>';
  try{
    const md = await apiReadme(owner, repo, branch);
    $('#readmeBox').innerHTML = md ? mdToHtml(md) : '<div class="muted">No README found</div>';
  }catch{
    $('#readmeBox').innerHTML = '<div class="muted">README not accessible</div>';
  }
}

function backToList(){
  $('#repoDetailCard').hidden = true;
  $('#repoListCard').hidden = false;
  $('#welcomeCard').hidden = false;
  currentRepo = null;
}

/* ---------- bind & boot ---------- */
function bind(){
  $('#langZh').onclick = ()=>{ lang='zh'; localStorage.setItem('xgit_lang',lang); applyI18n(); };
  $('#langEn').onclick = ()=>{ lang='en'; localStorage.setItem('xgit_lang',lang); applyI18n(); };
  $('#btnSign').onclick = async (e)=>{
    if(e.currentTarget.dataset.mode==='out') await signOutFlow(); else await signInFlow();
  };
  $('#btnSign2').onclick = async (e)=>{
    if(e.currentTarget.dataset.mode==='out') await signOutFlow(); else await signInFlow();
  };
  $('#btnRefresh').onclick = refreshFlow;
  $('#repoReload').onclick = loadRepos;
  $('#backToList').onclick = backToList;
}

async function boot(){
  applyI18n(); 
  bind();
  const me = await validateToken(); 
  setSignedUI(me);
  if(me) await loadRepos();
}

boot();
