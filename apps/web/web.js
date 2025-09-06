(function(){
  const $ = s=>document.querySelector(s);
  const LS_TOKEN = 'xgit_web_token';

  // —— 返回官网：同窗口 ——
  const SITE_URL = (() => {
    const h = location.hostname;
    if (h.startsWith('app.')) return location.protocol + '//' + h.replace(/^app\./,'');
    return '/apps/site/';
  })();
  $('#toSite').onclick = $('#btnBack')?.onclick = ()=>{ location.href = SITE_URL; };

  // —— 语言切换（只替换有翻译的；缺省中文）——
  $('#langZh').onclick = ()=>I18N.setLang('zh');
  $('#langEn').onclick = ()=>I18N.setLang('en');

  // ===== Token 登录 / 状态 =====
  async function fetchJson(url, token, headers={}){
    const tryOnce = async (authHeader) => {
      const r = await fetch(url, { headers: { 'Accept':'application/vnd.github+json', ...(authHeader?{Authorization: authHeader}:{}) , ...headers }});
      if (!r.ok) throw new Error('HTTP '+r.status);
      return r.json();
    };
    try{
      return await tryOnce(token ? 'Bearer '+token : null);
    }catch(e){
      if (token) return await tryOnce('token '+token);
      throw e;
    }
  }

  async function fetchUser(token){ return fetchJson('https://api.github.com/user', token); }

  async function loginFlow(){
    const tok = prompt('请输入 GitHub Token（建议 repo 权限，仅自用）：');
    if (!tok) return;
    localStorage.setItem(LS_TOKEN, tok.trim());
    await refreshStatus();
    await loadRepos();
  }

  function logout(){
    localStorage.removeItem(LS_TOKEN);
    $('#userAvatar').style.display='none';
    $('#userName').textContent='';
    $('#btnLogin').style.display='';
    $('#btnLogout').style.display='none';
    $('#repoList').innerHTML = `<div class="empty" data-i18n="pleaseLogin">请先登录</div>`;
    $('#treeList').innerHTML = `<div class="empty" data-i18n="selectRepoTip">选择一个仓库以浏览文件</div>`;
    $('#fileTitle').textContent = I18N.getLang()==='en'?'Viewer':'预览';
    $('#viewerBody').innerHTML = defaultWelcomeHtml();
  }

  async function refreshStatus(){
    const tok = localStorage.getItem(LS_TOKEN);
    if (!tok){ logout(); return; }
    try{
      const me = await fetchUser(tok);
      $('#userName').textContent = me.name || me.login || 'GitHub User';
      $('#userAvatar').src = (me.avatar_url||'') + '&s=48';
      $('#userAvatar').style.display='block';
      $('#btnLogin').style.display='none';
      $('#btnLogout').style.display='';
    }catch(e){
      alert('登录失效或 Token 无效，请重新登录。');
      logout();
    }
  }

  $('#btnLogin').onclick = $('#btnLogin2')?.onclick = loginFlow;
  $('#btnLogout').onclick = logout;

  // ===== 仓库列表 =====
  async function loadRepos(){
    const tok = localStorage.getItem(LS_TOKEN);
    if (!tok){ return; }
    const repos = await fetchJson('https://api.github.com/user/repos?per_page=100&sort=updated', tok);
    const html = repos.map(r=>{
      const full = r.full_name;
      const br = r.default_branch || 'main';
      const privacy = r.private ? '🔒' : '🌐';
      return `<div class="row" data-owner="${r.owner.login}" data-repo="${r.name}" data-branch="${br}">
        <div class="ellipsis">${privacy} ${full}</div>
        <div class="tag">${br}</div>
      </div>`;
    }).join('');
    $('#repoList').classList.remove('placeholder');
    $('#repoList').innerHTML = html || `<div class="empty">（无仓库）</div>`;
    // 绑定
    $('#repoList').querySelectorAll('.row').forEach(el=>{
      el.onclick = ()=> openRepo(el.dataset.owner, el.dataset.repo, el.dataset.branch);
    });
  }
  $('#btnReloadRepos').onclick = loadRepos;

  // ===== 文件树 & 预览 =====
  const state = { owner:null, repo:null, branch:'main', path:'' };

  function renderCrumbs(){
    const parts = state.path ? state.path.split('/').filter(Boolean) : [];
    const frag = [];
    frag.push(`<span class="crumb" data-i="">/</span>`);
    parts.forEach((p,i)=>{
      const sub = parts.slice(0,i+1).join('/');
      frag.push(`<span class="crumb" data-i="${sub}">${p}</span>`);
    });
    $('#crumbs').innerHTML = frag.join('');
    $('#crumbs').querySelectorAll('.crumb').forEach(el=>{
      el.onclick = ()=>{
        state.path = el.dataset.i || '';
        loadTree();
      };
    });
  }

  function extOf(name=''){ return (name.split('.').pop()||'').toLowerCase(); }
  function isImage(name){ return ['png','jpg','jpeg','gif','bmp','webp','svg'].includes(extOf(name)); }
  function isTextual(name){ return ['txt','md','markdown','json','js','ts','tsx','mjs','cjs','css','scss','less','html','htm','xml','yml','yaml','svg','py','rb','go','java','kt','rs','c','cpp','h','hpp','sh','bash','zsh','ini','toml','sql'].includes(extOf(name)); }

  async function openRepo(owner, repo, branch){
    state.owner=owner; state.repo=repo; state.branch=branch||'main'; state.path='';
    $('#repoTitle').textContent = `${owner}/${repo}`;
    $('#branchBadge').textContent = state.branch;
    renderCrumbs();
    await loadTree();
  }

  async function loadTree(){
    const tok = localStorage.getItem(LS_TOKEN);
    if (!tok){ return; }
    const base = `https://api.github.com/repos/${state.owner}/${state.repo}/contents`;
    const url = state.path ? `${base}/${encodeURIComponent(state.path).replace(/%2F/g,'/')}` : base;
    const items = await fetchJson(`${url}?ref=${encodeURIComponent(state.branch)}`, tok);
    const arr = Array.isArray(items) ? items : [items];
    // 目录在前，文件在后
    arr.sort((a,b)=> (a.type===b.type) ? a.name.localeCompare(b.name) : (a.type==='dir'?-1:1));
    $('#treeList').innerHTML = arr.map(it=>{
      const icon = it.type==='dir' ? '📁' : (isImage(it.name)?'🖼️':'📄');
      return `<div class="row" data-type="${it.type}" data-path="${it.path}" title="${it.path}">
        <div class="ellipsis">${icon} ${it.name}</div>
        ${it.type==='file'?`<div class="tag">${extOf(it.name)||'file'}</div>`:''}
      </div>`;
    }).join('');
    $('#treeList').querySelectorAll('.row').forEach(el=>{
      el.onclick = ()=>{
        const p = el.dataset.path;
        if (el.dataset.type==='dir'){
          state.path = p;
          renderCrumbs();
          loadTree();
        }else{
          openFile(p);
        }
      };
    });
  }

  async function openFile(path){
    const tok = localStorage.getItem(LS_TOKEN);
    const url = `https://api.github.com/repos/${state.owner}/${state.repo}/contents/${encodeURIComponent(path).replace(/%2F/g,'/')}?ref=${encodeURIComponent(state.branch)}`;
    const data = await fetchJson(url, tok, { });
    $('#fileTitle').textContent = path.split('/').pop();
    $('#btnDownloadRaw').style.display = data.download_url ? '' : 'none';
    $('#btnDownloadRaw').onclick = ()=>{
      // 对私仓：优先用 base64 dataURL 下载
      if (data.content && data.encoding==='base64'){
        const blob = b64ToBlob(data.content, mimeOf(path));
        const a = document.createElement('a');
        a.href = URL.createObjectURL(blob);
        a.download = path.split('/').pop();
        a.click();
        URL.revokeObjectURL(a.href);
      }else if (data.download_url){
        location.href = data.download_url; // 公仓可直接用
      }
    };

    if (isImage(path)){
      // 图片：用 dataURL（私仓也可）
      if (extOf(path)==='svg'){
        const svgText = atobSafe(data.content||'');
        $('#viewerBody').innerHTML = `<div class="image"><div>${svgText}</div></div>`;
      }else{
        const mime = mimeOf(path);
        const src = `data:${mime};base64,${(data.content||'').trim()}`;
        $('#viewerBody').innerHTML = `<div class="image"><img src="${src}" alt=""></div>`;
      }
      return;
    }

    if (isTextual(path) && data.content){
      const text = atobSafe(data.content);
      $('#viewerBody').innerHTML = `<pre>${escapeHtml(text)}</pre>`;
      return;
    }

    // 其它类型：给一个下载入口
    $('#viewerBody').innerHTML = `
      <div class="empty">
        该文件类型暂不支持预览。你可以点击右上角“⬇”下载原文件。
      </div>`;
  }

  // —— 顶部三个按钮：上一级 / 根目录 / 刷新树 ——
  $('#btnUp').onclick = ()=>{
    if (!state.path) return;
    const parts = state.path.split('/').filter(Boolean);
    parts.pop();
    state.path = parts.join('/');
    renderCrumbs();
    loadTree();
  };
  $('#btnRoot').onclick = ()=>{
    state.path = '';
    renderCrumbs();
    loadTree();
  };
  $('#btnRefreshTree').onclick = loadTree;

  // —— 小工具 —— 
  function escapeHtml(s){return String(s||'').replace(/[&<>]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]))}
  function atobSafe(b){ try{ return decodeURIComponent(escape(atob((b||'').replace(/\s/g,'')))); }catch{ return ''; } }
  function mimeOf(p){
    const e = (p.split('.').pop()||'').toLowerCase();
    const map = {png:'image/png',jpg:'image/jpeg',jpeg:'image/jpeg',gif:'image/gif',bmp:'image/bmp',webp:'image/webp',svg:'image/svg+xml'};
    return map[e] || 'application/octet-stream';
    }
  function b64ToBlob(b64, mime='application/octet-stream'){
    const bin = atob((b64||'').replace(/\s/g,''));
    const len = bin.length;
    const buf = new Uint8Array(len);
    for (let i=0;i<len;i++) buf[i] = bin.charCodeAt(i);
    return new Blob([buf], { type: mime });
  }
  function defaultWelcomeHtml(){
    return `
      <div class="welcome">
        <div class="hero-logo"><img src="./assets/logo.svg" alt="logo"></div>
        <h2 data-i18n="welcome">欢迎使用 XGit Web</h2>
        <p class="muted" data-i18n="tagline">随时随地，轻松管理你的 GitHub 仓库。</p>
        <div class="actions">
          <button id="btnLogin2" class="btn primary" data-i18n="loginByToken">用 Token 登录</button>
          <button id="btnBack" class="btn" data-i18n="backToSite">返回官网</button>
        </div>
      </div>`;
  }

  // —— 启动：刷新状态 & 仓库列表 —— 
  document.addEventListener('DOMContentLoaded', async ()=>{
    await refreshStatus();
    await loadRepos();
  });
})();