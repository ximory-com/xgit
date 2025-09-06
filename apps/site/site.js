// 语言与文本
const TEXT = {
  zh: {
    heroTitle: '随时随地，轻松管理你的 GitHub 仓库',
    heroSub: 'XGit 是一款专为移动端设计的轻量级 GitHub 客户端，让你在手机上也能流畅地查看、编辑与提交代码。',
    btnTry: '立即体验',
    btnRepo: 'GitHub 仓库',
    filterAll: '全部', filterDone: '已实现', filterWip: '开发中', filterPlan: '规划中',
    secBasic: '基础功能', secAdv: '增强功能', secPro: '高阶功能',
    cmpTitle: '与 GitHub 官方 App/Web 的差异', cmpFeature: '功能',
    posTitle: '产品定位',
    posText: '只把 仓库相关操作 做到极致：浏览 / 编辑 / Diff / 多文件一次提交 / 历史 / 分支 / 上传下载。不做 Issues/PR 等复杂协作台功能（必要时只读跳转）。',
    faq: [
      ['需要服务器吗？','不需要，全部在前端完成，直接对接 GitHub API。'],
      ['支持私有仓库吗？','支持，只需授权 Token（建议 scope: repo）。'],
      ['移动端体验如何？','采用单屏栈式导航（逐层进入），操作更顺手。']
    ],
    footCopy:'版权所有 © 2025 Ximory', footPoem:'与光同曜，随江河起落，合天地吐纳', footVer:'XGit v0.1.0',
    status:{done:'已实现', wip:'开发中', plan:'规划中'},
    searchPh:'搜索功能…'
  },
  en: {
    heroTitle: 'Manage your GitHub repos anywhere, anytime',
    heroSub: 'XGit is a lightweight GitHub client designed for mobile. Browse, edit and commit code seamlessly on the go.',
    btnTry: 'Try Now',
    btnRepo: 'GitHub Repo',
    filterAll: 'All', filterDone: 'Done', filterWip: 'WIP', filterPlan: 'Planned',
    secBasic: 'Basic Features', secAdv: 'Advanced', secPro: 'Pro',
    cmpTitle: 'Differences vs GitHub App/Web', cmpFeature: 'Feature',
    posTitle: 'Positioning',
    posText: 'Focus on repository operations only: browse / edit / diff / multi-file commit / history / branches / uploads. No complex collaborative features like Issues/PR (optional read-only jump).',
    faq: [
      ['Do I need a server?','No. Everything runs in the browser via GitHub API.'],
      ['Private repos?','Supported. Grant a token (recommended scope: repo).'],
      ['Mobile experience?','Single-screen stacked navigation, smooth and focused.']
    ],
    footCopy:'© 2025 Ximory', footPoem:'', footVer:'XGit v0.1.0',
    status:{done:'Done', wip:'WIP', plan:'Planned'},
    searchPh:'Search features…'
  }
};

// 对比表数据
const COMPARE = [
  { key:'repo',   zh:'仓库浏览', en:'Repo list', gh:'有',   xgit:'更快，移动优化 / Mobile optimized' },
  { key:'edit',   zh:'文件编辑', en:'File edit', gh:'有限', xgit:'Monaco（计划）/ Better editor' },
  { key:'commit', zh:'多文件提交', en:'Multi-file commit', gh:'无', xgit:'支持 / Supported' },
  { key:'zip',    zh:'ZIP 上传下载', en:'ZIP upload/download', gh:'无', xgit:'支持 / Supported' },
  { key:'ux',     zh:'移动端体验', en:'Mobile UX', gh:'一般', xgit:'栈式单屏 / Single-screen stack' }
];

// 功能清单（中英 + 状态）
const FEATURES = {
  basic: [
    { zh:{title:'仓库列表',desc:'登录后展示我的仓库，支持搜索与最近活跃排序。'},
      en:{title:'Repository list',desc:'Show my repos after login, with search & recent activity.'},
      status:'plan' },
    { zh:{title:'目录浏览',desc:'逐层进入（移动端单屏），桌面端三栏回退。'},
      en:{title:'Directory browser',desc:'Step into folders (mobile stack); desktop 3-pane.'},
      status:'plan' },
    { zh:{title:'文件查看/编辑（文本）',desc:'基础编辑可用，后续切换 Monaco。'},
      en:{title:'File view/edit (text)',desc:'Basic editing now; Monaco later.'},
      status:'plan' },
    { zh:{title:'提交（单/多文件）',desc:'Changes 勾选=暂存，一次提交所选。'},
      en:{title:'Commit (single/multi)',desc:'Stage with checkbox; commit selected at once.'},
      status:'plan' },
  ],
  adv: [
    { zh:{title:'Diff 视图（单文件）',desc:'original vs modified；支持行内高亮。'},
      en:{title:'Diff view (single file)',desc:'Original vs modified; inline highlight.'},
      status:'plan' },
    { zh:{title:'图片上传/预览',desc:'支持 png/jpg/svg 等上传与预览。'},
      en:{title:'Image upload/preview',desc:'Upload & preview png/jpg/svg etc.'},
      status:'plan' },
    { zh:{title:'ZIP 上传解包 / 打包下载',desc:'前端解压后逐文件写入；目录/勾选项打包下载。'},
      en:{title:'ZIP upload/unpack & download',desc:'Unpack in browser; zip current dir or selections.'},
      status:'plan' },
    { zh:{title:'国际化切换（中/英）',desc:'站点与 WebApp 一致，前端切换不刷新。'},
      en:{title:'i18n (zh/en)',desc:'Site & WebApp; switch without reload.'},
      status:'wip', date:'2025-09-05' },
  ],
  pro: [
    { zh:{title:'分支切换/新建',desc:'顶栏分支选择器；记忆最近分支。'},
      en:{title:'Branch switch/new',desc:'Topbar picker; recent branches.'},
      status:'plan' },
    { zh:{title:'提交历史 & 回滚',desc:'当前文件/目录的历史与一键回滚。'},
      en:{title:'History & rollback',desc:'Per-file/dir history and quick rollback.'},
      status:'plan' },
    { zh:{title:'搜索（文件名/内容）',desc:'先做文件名过滤，全文搜后置。'},
      en:{title:'Search (name/content)',desc:'Filename filter first; full-text later.'},
      status:'plan' },
  ]
};

const $ = s => document.querySelector(s);
const esc = s => String(s||'').replace(/[&<>]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]));

// 状态
let lang = localStorage.getItem('xgit_site_lang') ||
           ((navigator.language||'zh').toLowerCase().startsWith('zh') ? 'zh' : 'en');

// 渲染对比表
function renderCompare(){
  const tbody = $('#cmpBody');
  tbody.innerHTML = COMPARE.map(row=>{
    const feat = esc(row[lang]);
    return `<tr>
      <td>${feat}</td>
      <td>${esc(row.gh)}</td>
      <td>${esc(row.xgit)}</td>
    </tr>`;
  }).join('');
}

// 渲染功能卡片
function cardHTML(it){
  const t = it[lang];
  const badge = TEXT[lang].status[it.status] || it.status;
  const cls = it.status;
  const date = it.date ? `<span class="badge date">${esc(it.date)}</span>` : '';
  return `
  <article class="card">
    <div class="title">${esc(t.title)}</div>
    <div class="badges">
      <span class="badge ${cls}">${esc(badge)}</span>
      ${date}
    </div>
    ${t.desc ? `<div class="desc">${esc(t.desc)}</div>` : ''}
  </article>`;
}
function renderGroups(){
  const f = $('#statusFilter').value; // all/done/wip/plan
  const kw = ($('#searchBox').value || '').trim().toLowerCase();

  const renderGroup=(list, id)=>{
    const filtered = list.filter(it=>{
      if (f!=='all' && it.status!==f) return false;
      if (!kw) return true;
      const t = it[lang];
      return (t.title+' '+(t.desc||'')).toLowerCase().includes(kw);
    });
    $(id).innerHTML = filtered.map(cardHTML).join('') || `<div class="card"><div class="desc">Empty</div></div>`;
  };

  renderGroup(FEATURES.basic, '#group-basic');
  renderGroup(FEATURES.adv,   '#group-adv');
  renderGroup(FEATURES.pro,   '#group-pro');

  const all = [...FEATURES.basic, ...FEATURES.adv, ...FEATURES.pro];
  const done = all.filter(x=>x.status==='done').length;
  const pct  = all.length ? Math.round(done*100/all.length) : 0;
  $('#progressBar').style.width = pct + '%';
  $('#progressText').textContent = pct + '%';
}

// 文案替换
function renderText(){
  $('#heroTitle').textContent = TEXT[lang].heroTitle;
  $('#heroSub').textContent   = TEXT[lang].heroSub;
  $('#btnTry').textContent    = TEXT[lang].btnTry;
  $('#btnRepo').textContent   = TEXT[lang].btnRepo;

  $('#statusFilter').options[0].textContent = TEXT[lang].filterAll;
  $('#statusFilter').options[1].textContent = TEXT[lang].filterDone;
  $('#statusFilter').options[2].textContent = TEXT[lang].filterWip;
  $('#statusFilter').options[3].textContent = TEXT[lang].filterPlan;
  $('#searchBox').placeholder = TEXT[lang].searchPh;

  $('#secBasic').textContent = TEXT[lang].secBasic;
  $('#secAdv').textContent   = TEXT[lang].secAdv;
  $('#secPro').textContent   = TEXT[lang].secPro;

  $('#cmpTitle').textContent   = TEXT[lang].cmpTitle;
  $('#cmpFeature').textContent = TEXT[lang].cmpFeature;

  $('#posTitle').textContent = TEXT[lang].posTitle;
  $('#posText').innerHTML    = esc(TEXT[lang].posText).replace(/仓库相关操作|repository operations/g, m=>`<b>${m}</b>`);

  const ul = $('#faqList'); ul.innerHTML = '';
  TEXT[lang].faq.forEach(([q,a])=>{
    const li = document.createElement('li');
    li.innerHTML = `<b>${esc(q)}</b> ${esc(a)}`;
    ul.appendChild(li);
  });

  $('#footCopy').textContent = TEXT[lang].footCopy;
  $('#footPoem').textContent = TEXT[lang].footPoem;
  $('#footVer').textContent  = TEXT[lang].footVer;
}

// 绑定
function bind(){
  $('#langSwitch').value = lang;
  $('#langSwitch').onchange = ()=>{
    lang = $('#langSwitch').value;
    localStorage.setItem('xgit_site_lang', lang);
    renderAll();
  };
  $('#statusFilter').onchange = renderGroups;
  $('#searchBox').oninput = renderGroups;
}

function renderAll(){
  renderText();
  renderCompare();
  renderGroups();
}

bind(); renderAll();