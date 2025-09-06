const $  = s => document.querySelector(s);
const $$ = s => [...document.querySelectorAll(s)];

let lang = 'zh';

const dict = {
  zh: {
    // hero & ctas
    hero_title: '随时随地，轻松管理你的 GitHub 仓库',
    hero_sub  : 'XGit 是一款专为移动端设计的轻量级 GitHub 客户端，让你在手机上也能流畅地查看、编辑与提交代码。',
    cta_try   : '立即体验',
    cta_repo  : 'GitHub 仓库',

    // features
    feat_title: '基础功能',
    f_repo_list: '仓库列表',
    f_repo_list_desc: '登录后展示我的仓库，支持搜索与最近活跃排序。',
    f_tree: '目录浏览',
    f_tree_desc: '逐层进入（移动端单屏），桌面端三栏回退。',
    f_edit: '文件查看/编辑（文本）',
    f_edit_desc: '基础编辑可用，后续切换 Monaco。',

    // statuses
    status_done   : '已实现',
    status_wip    : '开发中',
    status_planned: '规划中',

    // compare
    cmp_title: '与 GitHub 官方 App/Web 的差异',
    cmp_feat : '功能',
    cmp_gh   : 'GitHub App/Web',
    cmp_xgit : 'XGit',
    compare_rows: [
      ['仓库浏览', '有', '更快，移动优化'],
      ['文件编辑', '有限', '更好的编辑器（计划 Monaco）'],
      ['多文件一次提交', '无', '支持'],
      ['ZIP 上传下载', '无', '支持'],
      ['移动端体验', '一般', '栈式单屏，更顺手']
    ],

    // position & faq
    pos_title: '产品定位',
    pos_desc : '只把 <b>仓库相关操作</b> 做到极致：浏览 / 编辑 / Diff / 多文件一次提交 / 历史 / 分支 / 上传下载。不做 Issues/PR 等复杂协作（必要时仅跳转）。',
    faq_q1:'需要服务器吗？', faq_a1:'不需要，全部在前端完成，直接对接 GitHub API。',
    faq_q2:'支持私有仓库吗？', faq_a2:'支持，只需授权 Token（建议 scope: repo）。',
    faq_q3:'移动端体验如何？', faq_a3:'采用单屏栈式导航（逐层进入），操作更顺手。'
  },

  en: {
    hero_title: 'Manage your GitHub repos anywhere, smoothly',
    hero_sub  : 'XGit is a lightweight GitHub client tailored for mobile, letting you browse, edit and commit right on your phone.',
    cta_try   : 'Open Web App',
    cta_repo  : 'GitHub Repo',

    feat_title: 'Core features',
    f_repo_list: 'Repository list',
    f_repo_list_desc: 'Show your repos after sign-in, with search and recent-activity sort.',
    f_tree: 'Tree browsing',
    f_tree_desc: 'Drill-down single screen on mobile; three-pane on desktop.',
    f_edit: 'File view / edit (text)',
    f_edit_desc: 'Basic editor; Monaco is planned.',

    status_done   : 'Done',
    status_wip    : 'In progress',
    status_planned: 'Planned',

    cmp_title: 'Differences vs GitHub App/Web',
    cmp_feat : 'Feature',
    cmp_gh   : 'GitHub App/Web',
    cmp_xgit : 'XGit',
    compare_rows: [
      ['Repo browsing', 'Available', 'Faster, mobile-optimized'],
      ['File editing', 'Limited', 'Better editor (Monaco planned)'],
      ['Multi-file commit', 'Not supported', 'Supported'],
      ['ZIP upload/download', 'Not supported', 'Supported'],
      ['Mobile experience', 'Average', 'Single-screen stack']
    ],

    pos_title: 'Product positioning',
    pos_desc : 'We focus on <b>repo operations</b> only: browse / edit / diff / multi-file commit / history / branches / upload & download. Collaboration (Issues/PR) is out of scope (link-out when needed).',
    faq_q1:'Do I need a server?', faq_a1:'No. Everything is done on the client, talking to GitHub API directly.',
    faq_q2:'Private repos supported?', faq_a2:'Yes. Just grant a token (scope: repo recommended).',
    faq_q3:'How is mobile UX?',   faq_a3:'Single-screen stacked navigation feels smoother.'
  }
};

function applyText(){
  $$('[data-i18n]').forEach(el=>{
    const k = el.getAttribute('data-i18n');
    const v = dict[lang][k];
    if (v == null) return;
    // 允许部分字段包含 <b>…</b>
    if (/<\/?[a-z][\s\S]*>/i.test(v)) el.innerHTML = v;
    else el.textContent = v;
  });
}

function renderCompare(){
  const rows = dict[lang].compare_rows || [];
  $('#cmpBody').innerHTML = rows.map(r=>{
    const [f, gh, xg] = r;
    return `<tr><td>${esc(f)}</td><td>${esc(gh)}</td><td>${esc(xg)}</td></tr>`;
  }).join('');
}

function esc(s){ return String(s??'').replace(/[&<>"]/g,c=>({ '&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;' }[c])); }

function restoreLang(){
  const saved = localStorage.getItem('xgit_site_lang');
  lang = (saved || (navigator.language||'zh').toLowerCase().startsWith('zh') ? 'zh' : 'en');
  $('#langSwitch').value = lang;
}

function bind(){
  $('#langSwitch').onchange = ()=>{
    lang = $('#langSwitch').value;
    localStorage.setItem('xgit_site_lang', lang);
    applyText();
    renderCompare();
  };
}

function boot(){
  restoreLang();
  bind();
  applyText();
  renderCompare();
}
boot();