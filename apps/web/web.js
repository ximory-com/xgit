import { api } from './github.js';
import { getStoredToken, setStoredToken, clearToken } from './auth.js';

const els = {
  home: document.getElementById('home'),
  homeAuthed: document.getElementById('homeAuthed'),
  hiUser: document.getElementById('hiUser'),
  recentRepos: document.getElementById('recentRepos'),

  loginBtn: document.getElementById('loginBtn'),
  logoutBtn: document.getElementById('logoutBtn'),
  goLogin: document.getElementById('goLogin'),

  repos: document.getElementById('repos'),
  reposList: document.getElementById('reposList'),

  tree: document.getElementById('tree'),
  treeList: document.getElementById('treeList'),
  breadcrumb: document.getElementById('breadcrumb'),
  navUp: document.getElementById('navUp'),
  navRoot: document.getElementById('navRoot'),
  navRefresh: document.getElementById('navRefresh'),

  editor: document.getElementById('editor'),
  tabs: document.querySelectorAll('.tab'),
  filePane: document.getElementById('filePane'),
  changesPane: document.getElementById('changesPane'),
  textViewer: document.getElementById('textViewer'),
  toggleEdit: document.getElementById('toggleEdit'),
  discardLocal: document.getElementById('discardLocal'),
  filePath: document.getElementById('filePath'),
  binaryHint: document.getElementById('binaryHint'),
  imageWrap: document.getElementById('imageWrap'),
  imagePreview: document.getElementById('imagePreview'),

  changesList: document.getElementById('changesList'),
  commitMsg: document.getElementById('commitMsg'),
  commitAll: document.getElementById('commitAll'),
};

const state = {
  token: null,
  user: null,
  _repos: [],
  repo: null,              // { owner, repo, default_branch }
  branch: 'main',
  pathStack: [''],
  current: null,           // { path, sha, type }
  changes: {},             // { path: { old, new, staged:true } }
};

init();

function $(s){ return document.querySelector(s); }
function esc(s){ return String(s||'').replace(/[&<>"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c])); }
const btoaSafe = s => btoa(unescape(encodeURIComponent(s)));
const atobSafe = b => decodeURIComponent(escape(atob(b)));

async function init(){
  bind();
  switchPanel('home');

  // 恢复 token
  const stored = getStoredToken();
  if (stored?.access_token) {
    state.token = stored.access_token;
    try {
      state.user = await api.me(state.token);
      renderAuth();
      await loadRepos();
    } catch (e) {
      console.warn(e); clearToken();
      state.token = null; state.user = null;
      renderAuth();
    }
  } else {
    renderAuth();
  }
}

function bind(){
  // 登录/退出
  els.loginBtn.onclick = onTokenLogin;
  els.goLogin.onclick = onTokenLogin;
  els.logoutBtn.onclick = ()=>{ clearToken(); location.reload(); };

  // Home 按钮
  $('#openRepos')?.addEventListener('click', ()=> switchPanel('repos'));
  $('#openLast')?.addEventListener('click', openLastRepo);

  // Tabs（右栏）
  els.tabs.forEach(tab=>{
    tab.addEventListener('click', ()=>{
      els.tabs.forEach(t=> t.classList.toggle('active', t===tab));
      const name = tab.dataset.tab;
      document.querySelectorAll('.pane').forEach(p => p.classList.toggle('active', p.dataset.pane===name));
    });
  });

  // 目录导航
  els.navUp.onclick = ()=> {
    if (state.pathStack.length>1) { state.pathStack.pop(); loadTree(curPath()); }
  };
  els.navRoot.onclick = ()=> { state.pathStack = ['']; loadTree(''); };
  els.navRefresh.onclick = ()=> loadTree(curPath());

  // 编辑器控件
  els.toggleEdit.onclick = toggleEditMode;
  els.discardLocal.onclick = discardCurrentLocal;
  els.textViewer.addEventListener('input', onEditorInput);

  // 提交
  els.commitAll.onclick = commitSelected;
}

function renderAuth(){
  const authed = !!state.user;
  els.loginBtn.hidden = authed;
  els.logoutBtn.hidden = !authed;
  if (authed) {
    els.hiUser.textContent = '@' + state.user.login;
    els.homeAuthed.hidden = false;
  } else {
    els.homeAuthed.hidden = true;
  }
}

async function onTokenLogin(){
  const t = prompt('粘贴你的 GitHub Token（scope: repo）');
  if (!t) return;
  try{
    state.token = t.trim();
    const me = await api.me(state.token);
    state.user = me;
    setStoredToken(state.token);
    renderAuth();
    await loadRepos();
    switchPanel('repos');
  }catch(e){
    alert('Token 无效或权限不足：' + e.message);
    state.token=null; clearToken();
  }
}

async function loadRepos(){
  const repos = await api.repos(state.token, 'all');
  state._repos = repos;
  els.reposList.innerHTML = repos.map(r=>`
    <div class="item" data-owner="${esc(r.owner.login)}" data-repo="${esc(r.name)}" data-branch="${esc(r.default_branch||'main')}">
      ${esc(r.full_name)}
    </div>`).join('');
  els.reposList.querySelectorAll('.item').forEach(el=>{
    el.onclick = ()=>{
      const r = {
        owner: el.dataset.owner,
        repo: el.dataset.repo,
        default_branch: el.dataset.branch,
        name: `${el.dataset.owner}/${el.dataset.repo}`
      };
      onRepoClick(r);
    };
  });

  // Home 最近仓库
  renderHomeRecent();
  // Home 上次仓库按钮
  const last = JSON.parse(localStorage.getItem('xgit_last_repo')||'null');
  const btnLast = document.getElementById('openLast');
  if (btnLast) btnLast.disabled = !last;
}

function renderHomeRecent(){
  if (!els.recentRepos) return;
  const top = state._repos.slice(0,5);
  els.recentRepos.innerHTML = top.map(r=>`
    <div class="item" data-owner="${esc(r.owner.login)}" data-repo="${esc(r.name)}" data-branch="${esc(r.default_branch||'main')}">
      ${esc(r.full_name)}
    </div>`).join('');
  els.recentRepos.querySelectorAll('.item').forEach(el=>{
    el.onclick = ()=>{
      const r = {
        owner: el.dataset.owner,
        repo: el.dataset.repo,
        default_branch: el.dataset.branch,
        name: `${el.dataset.owner}/${el.dataset.repo}`
      };
      onRepoClick(r);
    };
  });
}

async function onRepoClick(r){
  state.repo = r;
  state.branch = r.default_branch || 'main';
  localStorage.setItem('xgit_last_repo', JSON.stringify(r));
  state.pathStack = [''];
  await loadTree('');
  switchPanelDesktopAware('tree'); // 移动端进 tree；桌面三栏都显示
}

function curPath(){ return state.pathStack[state.pathStack.length-1] || ''; }

async function loadTree(path){
  if (path!==undefined) {
    if (state.pathStack[state.pathStack.length-1]!==path){
      state.pathStack.push(path);
    }
  }
  const p = curPath();
  els.breadcrumb.textContent = '/' + p;
  const { owner, repo } = state.repo;

  let list = await api.listPath(state.token, { owner, repo, path: p, ref: state.branch });
  if (!Array.isArray(list)) list = [list];

  // 目录优先、其后文件
  list.sort((a,b)=>{
    if (a.type===b.type) return a.name.localeCompare(b.name);
    return a.type==='dir' ? -1 : 1;
  });

  els.treeList.innerHTML = list.map(i=>`
    <div class="item" data-type="${i.type}" data-path="${esc(i.path)}">
      ${i.type==='dir' ? '📁' : '📄'} ${esc(i.name)}
    </div>`).join('');

  els.treeList.querySelectorAll('.item').forEach(el=>{
    el.onclick = ()=>{
      const type = el.dataset.type;
      const path = el.dataset.path;
      if (type==='dir') {
        state.pathStack.push(path);
        loadTree(path);
      } else {
        openFile(path);
      }
    };
  });
}

async function openFile(path){
  const { owner, repo } = state.repo;
  const f = await api.getFile(state.token, { owner, repo, path, ref: state.branch });
  state.current = { path, sha: f.sha, type: f.type };

  els.filePath.textContent = path;
  els.toggleEdit.disabled = false;
  els.discardLocal.disabled = false;

  // 根据类型展示
  const isText = /^text\/|application\/(json|xml|javascript)/.test(f.type||'')
               || /\.(md|txt|json|ya?ml|js|ts|html|css|mdx|toml|ini|py|rb|go|rs|java|kt|c|cpp|h|php|sh)$/i.test(path);
  const isImage = /\.(png|jpg|jpeg|gif|webp|svg)$/i.test(path);

  els.binaryHint.hidden = true;
  els.imageWrap.hidden = true;
  els.textViewer.hidden = false;

  if (isImage) {
    els.textViewer.hidden = true;
    els.imageWrap.hidden = false;
    if (f.content) {
      const raw = atobSafe(f.content);
      // 若是 svg 可直接显示文本；其余走 dataURL
      if (/\.svg$/i.test(path)) {
        const blob = new Blob([raw], { type: 'image/svg+xml' });
        els.imagePreview.src = URL.createObjectURL(blob);
      } else {
        els.imagePreview.src = `data:${f.type||'image/*'};base64,${f.content}`;
      }
    } else {
      els.imagePreview.src = '';
    }
    return;
  }

  if (!isText) {
    els.textViewer.hidden = true;
    els.binaryHint.hidden = false;
    return;
  }

  // 文本内容
  const text = f.content ? atobSafe(f.content) : '';
  els.textViewer.value = text;
  els.textViewer.disabled = false;

  // 初始化 changes 基线
  const ch = state.changes[path];
  if (!ch) {
    state.changes[path] = { old: text, new: text, staged: false };
  } else {
    // 如果之前已经改过，维持之前的 new
    els.textViewer.value = ch.new;
  }
  refreshChangesUI();
  switchEditorTab('file');
}

function toggleEditMode(){
  if (els.textViewer.disabled) return; // 纯 viewer 时忽略
  // 简化：textarea 无只读模式，按钮作为提示
  alert('直接在文本框编辑，修改会自动加入 Changes。');
}

function onEditorInput(){
  if (!state.current?.path) return;
  const p = state.current.path;
  const entry = state.changes[p] || (state.changes[p] = { old: els.textViewer.value, new: els.textViewer.value, staged:false });
  entry.new = els.textViewer.value;
  entry.staged = entry.new !== entry.old;
  if (entry.new === entry.old) delete state.changes[p];
  refreshChangesUI();
}

function discardCurrentLocal(){
  if (!state.current?.path) return;
  const p = state.current.path;
  const entry = state.changes[p];
  if (!entry) return;
  if (!confirm(`撤销本地更改：\n${p}`)) return;
  els.textViewer.value = entry.old;
  delete state.changes[p];
  refreshChangesUI();
}

function refreshChangesUI(){
  const entries = Object.entries(state.changes).filter(([_,c])=> c && c.new!==c.old);
  const box = els.changesList;
  if (entries.length===0) {
    box.innerHTML = `<div class="muted">没有修改</div>`;
  } else {
    box.innerHTML = entries.map(([path,c])=>`
      <div class="change-row">
        <label><input type="checkbox" data-path="${esc(path)}" ${c.staged?'checked':''}> ${esc(path)}</label>
        <div>
          <button class="btn-mini" data-edit="${esc(path)}">编辑</button>
          <button class="btn-mini" data-discard="${esc(path)}">撤销</button>
          <button class="btn-mini" data-commit-one="${esc(path)}">提交此文件</button>
        </div>
      </div>`).join('');
  }

  // 绑定
  box.querySelectorAll('input[type=checkbox]').forEach(chk=>{
    chk.onchange = ()=> { const p=chk.dataset.path; if (state.changes[p]) state.changes[p].staged = chk.checked; };
  });
  box.querySelectorAll('[data-edit]').forEach(b=> b.onclick = ()=>{
    const p = b.dataset.edit;
    const c = state.changes[p];
    if (c) {
      // 打开并显示 new
      state.current = { ...(state.current||{}), path:p };
      els.filePath.textContent = p;
      els.textViewer.value = c.new;
      switchEditorTab('file');
    }
  });
  box.querySelectorAll('[data-discard]').forEach(b=> b.onclick = ()=>{
    const p = b.dataset.discard;
    const c = state.changes[p];
    if (!c) return;
    if (!confirm(`撤销本地更改：\n${p}`)) return;
    if (state.current?.path === p) {
      els.textViewer.value = c.old;
    }
    delete state.changes[p];
    refreshChangesUI();
  });
  box.querySelectorAll('[data-commit-one]').forEach(b=> b.onclick = async ()=>{
    const p = b.dataset.commitOne;
    const c = state.changes[p];
    if (!c) return;
    await commitFiles([[p,c]], els.commitMsg.value || 'update via XGit');
    delete state.changes[p];
    refreshChangesUI();
  });
}

async function commitSelected(){
  const msg = els.commitMsg.value || 'update via XGit';
  const staged = Object.entries(state.changes).filter(([_,c])=> c.staged && c.new!==c.old);
  if (staged.length===0) return alert('没有选中文件');
  await commitFiles(staged, msg);
  staged.forEach(([p])=> delete state.changes[p]);
  refreshChangesUI();
  alert(`提交成功：${staged.length} 个文件`);
}

async function commitFiles(entries, message){
  const { owner, repo } = state.repo;
  for (const [path, c] of entries) {
    const base64 = btoaSafe(c.new);
    try{
      // 取最新 sha（避免并发覆盖）
      let sha = null;
      try {
        const f = await api.getFile(state.token, { owner, repo, path, ref: state.branch });
        sha = f?.sha || null;
      } catch(_) { /* new file case */ }

      const res = await api.putFile(state.token, {
        owner, repo, path, message, contentBase64: base64, sha, branch: state.branch
      });
      // 更新当前文件 sha
      if (state.current?.path === path && res?.content?.sha) {
        state.current.sha = res.content.sha;
      }
    }catch(e){
      alert(`提交失败：${path}\n${e.message}`);
      throw e;
    }
  }
}

function switchPanel(name){
  document.querySelectorAll('.panel').forEach(p=> p.classList.toggle('active', p.id===name));
}
function switchPanelDesktopAware(name){
  if (window.matchMedia('(min-width:1000px)').matches) {
    // 三栏：全部显示，不切换
    return;
  }
  switchPanel(name);
}
function switchEditorTab(name){
  document.querySelectorAll('.tab').forEach(t=> t.classList.toggle('active', t.dataset.tab===name));
  document.querySelectorAll('.pane').forEach(p=> p.classList.toggle('active', p.dataset.pane===name));
}

function openLastRepo(){
  const last = JSON.parse(localStorage.getItem('xgit_last_repo')||'null');
  if (!last) return;
  onRepoClick(last);
}